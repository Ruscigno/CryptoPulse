# Stock Screener — Design

**Date:** 2026-06-16
**Status:** Approved (pending written-spec review)

## 1. Goal

Build a stock screener that:

1. Collects market data from Yahoo Finance for a configured list of stocks.
2. Evaluates each stock across multiple timeframes using three indicators.
3. Exposes an HTTP endpoint that returns the (stock, timeframe) rows currently
   sitting at a recent extreme, along with rich indicator data for each row.

The project today is a single `main.go` that fetches OHLCV via `go-quote` and
writes one latest row per stock into a Postgres `intraday_prices` table. This
design replaces that skeleton with a structured, testable system.

## 2. Core concepts

### 2.1 Indicators

All indicators are computed per (symbol, timeframe) on **closed bars only**
(the still-forming current bar is dropped — "wait for timeframe close").

| Indicator | Formula | Params (from config) |
|---|---|---|
| **RSI** | Wilder's RSI on close | length 14, source close, smoothing SMA 14 (BB StdDev 2, used only if smoothing type = Bollinger), divergence off |
| **Volume Oscillator** | `100 · (EMA_short(vol) − EMA_long(vol)) / EMA_long(vol)` | short EMA 5, long EMA 10 |
| **Distance from MA** | `(close − MA) / MA · 100` (percent) | source close, MA type EMA, length 200, calculation percent |

The RSI smoothing line (SMA 14 of RSI) and BB StdDev are captured in config for
completeness. Screening, peaks/valleys, and trend operate on the **base RSI(14)**
series. The smoothing line is reserved for future use and not required for v1.

### 2.2 Per-indicator output

For each indicator the result carries:

- **latest** — value at the last closed bar.
- **trend** — slope over the last `N` bars (default 3): `rising` / `falling` /
  `flat`. `flat` when the change is within a small epsilon.
- **peaks** — the 3 most recent confirmed peak pivots (`{value, time}`).
- **valleys** — the 3 most recent confirmed valley pivots (`{value, time}`).
- **zone** — `high` / `low` / `neutral` (see screening rule).
- **triggered** — boolean, `true` when `zone != neutral`.

### 2.3 Peak / valley detection (pivots)

- A bar is a **peak** if its indicator value is strictly greater than the `w`
  bars on each side (pivot window `w`, default 3); a **valley** if strictly less.
- Confirming a pivot requires `w` bars after it, so the most recent `w` bars are
  not yet classified. This is intentional: the "current value" is tested
  *against* confirmed pivots.
- **Lookback:** scan back over a minimum duration (`peak_lookback`, default
  `3mo`) converted to a bar count per timeframe. If fewer than 3 pivots are found
  in that window, **extend the scan further back** until 3 pivots are found or
  history runs out. (On the monthly timeframe 3 months ≈ 3 bars, so it always
  extends — fine, since full history is fetched.)
- "peaks/valleys" = the **3 most recent** confirmed pivots (consistent with the
  screening rule), not the 3 most extreme.

### 2.4 Screening (match) rule

For one indicator, let `P` = last 3 peak values, `V` = last 3 valley values,
`c` = current (latest closed) value:

- **at a high** if `c ≥ min(P)`  → `zone = high`
- **at a low** if `c ≤ max(V)`  → `zone = low`
- otherwise `zone = neutral`
- the indicator **triggers** if `zone != neutral`.

A **(stock, timeframe) row qualifies** based on `match`:

- `any` (default) — at least one of the 3 indicators triggers.
- `all` — all three trigger.
- `min:N` — at least N of 3 trigger.

A qualifying row always reports **all three** indicators; `triggered` lists which
ones fired.

## 3. Timeframes

Evaluated set: `15m, 30m, 1h, 4h, 1d, 3d, 1wk, 1mo` (here `1m` = one month).

- **Native** (fetched from Yahoo): `15m, 30m, 1h, 1d, 1wk, 1mo`.
- **Derived** (resampled at query time, never stored): `4h` ← `1h`, `3d` ← `1d`.

Yahoo history limits to respect during collection: 15m/30m ≈ 60 days,
1h ≈ 730 days, daily/weekly/monthly ≈ full history.

**Resampling:** bucket native bars → `open` = first, `high` = max, `low` = min,
`close` = last, `volume` = sum. 4h groups 1h bars (4 per bucket); 3d groups 1d
bars (3 per bucket). Buckets anchored to fixed UTC boundaries. Session-aware
alignment is a possible later refinement.

## 4. Architecture

Chosen approach: **DB collector + on-demand compute**. A background collector
keeps native-timeframe OHLCV in Postgres; the HTTP endpoint reads from the DB,
resamples derived timeframes, computes indicators, and screens — no Yahoo call
in the request path.

### 4.1 Packages

Each package is small, single-purpose, and independently testable.

| Package | Responsibility | Depends on |
|---|---|---|
| `config` | Load/validate `config.yaml`; parse durations (`3mo`) | — |
| `datasource/yahoo` | Fetch native-TF OHLCV (wraps `go-quote`); map TF strings → Yahoo intervals; respect history limits | — |
| `storage` | Postgres: schema, upsert bars, query by (symbol, TF, range) | — |
| `resample` | Aggregate native bars → derived TFs (pure) | — |
| `indicators` | RSI, volume osc, distance-from-MA + EMA/SMA helpers (pure) | — |
| `extrema` | Pivot peak/valley detection; "last 3 within ≥ lookback" (pure) | — |
| `screener` | Orchestrate: load → resample → indicators → extrema → trend → match → rows | all above |
| `api` | HTTP server, `/screen` + `/healthz` handlers, JSON | screener |
| `collector` | Scheduler: periodically fetch native TFs & upsert | yahoo, storage |
| `cmd` / `main` | Wiring; subcommands `serve` / `collect` | all |

### 4.2 Data flow

**Collection (background):** for each (symbol, native TF) → Yahoo fetch →
upsert into `bars` (closed bars only), on the `collector.refresh` cadence.
Replaces today's broken infinite re-enqueue worker. Uses a worker pool with a
Yahoo rate limiter.

**Query (`GET /screen`):** for each (symbol, requested TF) → load native bars
from DB → resample if derived → compute 3 indicators → detect last-3 pivots +
trend on each → apply match rule → include row if it qualifies → JSON response.

### 4.3 Run modes

Single binary with subcommands:

- `stock-screener serve --config config.yaml` — HTTP server; runs the collector
  in-process when `collector.enabled` is true.
- `stock-screener collect --config config.yaml` — standalone collection (for
  cron / one-shot backfills).

## 5. Database schema

Replace `intraday_prices` with a `bars` table storing **native timeframes only**;
derived timeframes are computed at query time.

```sql
CREATE TABLE bars (
  symbol    TEXT             NOT NULL,
  timeframe TEXT             NOT NULL,   -- 15m,30m,1h,1d,1wk,1mo
  ts        TIMESTAMPTZ      NOT NULL,   -- bar open time, UTC
  open      DOUBLE PRECISION NOT NULL,
  high      DOUBLE PRECISION NOT NULL,
  low       DOUBLE PRECISION NOT NULL,
  close     DOUBLE PRECISION NOT NULL,
  volume    DOUBLE PRECISION NOT NULL,
  PRIMARY KEY (symbol, timeframe, ts)
);
CREATE INDEX bars_symbol_tf_ts ON bars (symbol, timeframe, ts DESC);

CREATE TABLE collector_state (
  symbol    TEXT        NOT NULL,
  timeframe TEXT        NOT NULL,
  last_ts   TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (symbol, timeframe)
);
```

Upsert via `INSERT ... ON CONFLICT (symbol, timeframe, ts) DO UPDATE`.
`collector_state.last_ts` enables incremental fetches.

## 6. Configuration

Everything tunable lives in one `config.yaml` (replaces `stocks.json`). DB
credentials stay in environment variables (secrets):
`DB_USER, DB_PASSWORD, DB_HOST, DB_PORT, DB_NAME`.

```yaml
server:
  port: 8080

collector:
  enabled: true
  use_closed_bars_only: true
  refresh:
    intraday: 15m   # 15m, 30m, 1h
    daily: 6h       # 1d, 1wk, 1mo

stocks: [AAPL, GOOGL, MSFT, TSLA, AMZN]

timeframes:          # native = from Yahoo, derived = resampled
  - 15m   # native
  - 30m   # native
  - 1h    # native
  - 4h    # derived from 1h
  - 1d    # native
  - 3d    # derived from 1d
  - 1wk   # native
  - 1mo   # native

screening:
  match: any            # any | all | min:N
  pivot_window: 3
  trend_lookback: 3
  peaks_to_show: 3
  peak_lookback: 3mo    # minimum scan window; extends until 3 pivots found

indicators:
  rsi:
    length: 14
    source: close
    smoothing: { type: SMA, length: 14, bb_stddev: 2 }
    # peak_lookback: 3mo   # optional per-indicator override
  volume_oscillator:
    short_length: 5
    long_length: 10
  distance_from_ma:
    source: close
    ma_type: EMA
    length: 200
    calculation: percent
```

## 7. API contract

### `GET /screen`

All query params optional; each defaults to `config.yaml`.

| Param | Example | Default |
|---|---|---|
| `symbols` | `AAPL,MSFT` | all config stocks |
| `timeframes` | `1d,1h` | all config timeframes |
| `match` | `any` \| `all` \| `min:2` | config (`any`) |
| `indicators` | `rsi,distance_from_ma` | all three |

Errors: unknown symbol/timeframe → `400`; DB failure → `500`. Per-item issues
never fail the request — they go into `warnings` with a `200`.

### `GET /healthz`

Liveness check (DB ping). Returns `200` / `503`.

### Response shape

`results` holds only qualifying rows; every row shows all three indicators with
`triggered` flagging which fired. Results sorted by symbol, then config
timeframe order.

```json
{
  "as_of": "2026-06-16T20:00:00Z",
  "criteria": { "match": "any", "symbols": 5, "timeframes": ["15m","30m","1h","4h","1d","3d","1wk","1mo"] },
  "results": [
    {
      "symbol": "AAPL",
      "timeframe": "1d",
      "bar_time": "2026-06-15T00:00:00Z",
      "price": 198.42,
      "triggered": ["rsi", "distance_from_ma"],
      "indicators": {
        "rsi": {
          "latest": 28.3, "trend": "rising", "zone": "low", "triggered": true,
          "peaks":   [{"value":72.1,"time":"2026-05-02T00:00:00Z"}, {"value":70.5,"time":"..."}, {"value":68.9,"time":"..."}],
          "valleys": [{"value":27.5,"time":"2026-06-15T00:00:00Z"}, {"value":29.0,"time":"..."}, {"value":31.2,"time":"..."}]
        },
        "volume_oscillator": { "latest": 12.4, "trend": "flat", "zone": "neutral", "triggered": false, "peaks": [], "valleys": [] },
        "distance_from_ma": { "latest": -8.2, "trend": "falling", "zone": "low", "triggered": true, "peaks": [], "valleys": [] }
      }
    }
  ],
  "warnings": [
    { "symbol": "TSLA", "timeframe": "15m", "message": "insufficient_data: need 200 bars for EMA200, have 140" }
  ]
}
```

Field notes:

- **zone** — `high` (`c ≥ min(last-3 peaks)`) / `low` (`c ≤ max(last-3 valleys)`) / `neutral`.
- **triggered** (indicator) — `zone != neutral`. **triggered** (row) — array of fired indicators.
- **trend** — `rising`/`falling`/`flat` over last `trend_lookback` bars.
- **peaks/valleys** — 3 most recent pivots, `{value, time}`.
- **price** — last closed bar's close, for context.

## 8. Error handling

- Invalid config or unreachable DB at startup → fail fast.
- Per-(symbol, TF) collection error → log, back off, skip, continue others;
  retry transient Yahoo errors with backoff.
- Insufficient history for an indicator (e.g. < 200 bars for EMA₂₀₀, or too few
  bars for pivots) → that indicator reported as `insufficient_data` in
  `warnings` rather than failing the row; the row can still qualify on the
  other indicators.
- `/screen` returns partial results plus a `warnings` array.

## 9. Testing (TDD)

- **Pure unit tests** against known reference values: `indicators` (RSI / vol osc
  / distance), `extrema` (pivot detection + last-3 + lookback extension),
  `resample` (4h/3d bucketing), `screener` match rule, `trend`, `config`
  (parsing + duration `3mo`).
- **Storage integration test** against a test Postgres: upsert/query roundtrip,
  `ON CONFLICT` upsert, incremental `collector_state`.
- **API handler test** with a fixture-backed fake storage: response shape,
  filtering by params, warnings on insufficient data, error codes.

## 10. Out of scope (v1 / YAGNI)

- Precomputed/materialized screening results (can layer caching on later).
- Authentication / rate limiting on the HTTP API.
- Session-aware resample alignment (use fixed UTC buckets for now).
- RSI smoothing/Bollinger outputs in the response (params captured only).
- Web UI (API is JSON-only).
