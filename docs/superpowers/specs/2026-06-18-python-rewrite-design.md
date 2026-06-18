# Stock Screener — Python Rewrite — Design

**Date:** 2026-06-18
**Status:** Approved (design)

## 1. Goal & context

Rewrite the stock screener from Go to Python, keeping behavior feature-for-feature
identical, while (a) replacing hand-rolled pieces with proven libraries and
(b) upgrading peak/valley detection to a smoothing + `scipy.signal.find_peaks`
(prominence + distance) approach.

The merged Go implementation and the existing specs are the **functional
reference** — requirements are already settled:

- Reference specs: `docs/superpowers/specs/2026-06-16-stock-screener-design.md`
  (core), `docs/superpowers/specs/2026-06-17-matches-endpoint-design.md`
  (`/matches`).
- Behavior to preserve: `collect` + `serve`; endpoints `/screen`, `/matches`,
  `/healthz`; `config.yaml`; Postgres `bars` schema; indicators RSI / volume
  oscillator / distance-from-MA; multi-timeframe with `4h`/`3d` resampled from
  `1h`/`1d`; screening rule (current value vs the last-3 peaks/valleys); warnings
  for insufficient data; closed-bars-only collection; incremental fetch.

The peak/valley detection is **improved** (not a straight port) per the
`2026-06-18` brainstorm.

## 2. Stack

- **Web:** FastAPI + uvicorn (pydantic response models; OpenAPI for free).
- **Data:** `yfinance` for Yahoo OHLCV.
- **DB:** SQLAlchemy Core (no ORM) over the existing Postgres `screener` DB.
- **Indicators:** pandas + `pandas-ta` (Wilder RSI from pandas-ta; volume
  oscillator and distance-from-MA via pandas `ewm`).
- **Detection:** `scipy.signal.find_peaks` (prominence + distance) on a smoothed
  series.
- **CLI:** Typer (`collect`, `serve`).
- **Tooling:** `pyproject.toml`; pytest, ruff, mypy.
- **Python:** 3.12 (matches the iac CI host).

Data continues to live in the shared Postgres `tb-postgres` (`localhost:5433`,
DB `screener`, role `screener_app`); DB credentials come from environment
variables (`.env`, git-ignored) exactly as today: `DB_USER`, `DB_PASSWORD`,
`DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_SSLMODE`.

## 3. Module layout (`stock_screener/` package)

| Module | Responsibility | Key libs |
|---|---|---|
| `config.py` | Load + validate `config.yaml` (pydantic models); parse durations (`3mo`, `15m`, …) | pydantic, pyyaml |
| `timeframes.py` | TF registry: native vs derived, yfinance interval, parent, bar duration, bucket start | — |
| `datasource.py` | Fetch OHLCV via yfinance; map TF→interval; incremental (`start=last_bar`); drop the still-forming bar | yfinance, pandas |
| `storage.py` | `bars` table; `upsert_bars` (ON CONFLICT), `get_bars`, `last_bar_time`, `ping`, `migrate`; pooled engine | SQLAlchemy Core |
| `resample.py` | Build derived TFs (4h←1h, 3d←1d) via `df.resample().agg(OHLCV)`; drop incomplete trailing bucket when closed-bars-only | pandas |
| `indicators.py` | `rsi(close, n)` (Wilder via pandas-ta), `volume_oscillator(vol, s, l)`, `distance_from_ma(close, ma_type, n)` → pandas Series | pandas, pandas-ta |
| `detection.py` | `smooth(series, period)` (ewm; 1=identity); `find_extrema(series, min_prominence, min_distance)` → peaks/valleys indices+values | scipy, pandas |
| `rule.py` | `classify(current, peaks, valleys)`→zone; `qualifies(triggered, requested, match)`; `valid_match(mode)` | — |
| `screener.py` | Orchestrate per (symbol, tf): load→resample→indicator→smooth+detect→zone/trend→row; aggregate matches | — |
| `api.py` | FastAPI app + routes `/screen`, `/matches`, `/healthz`; pydantic response models; shared request parsing/validation | FastAPI |
| `collector.py` | `collect_once` (all symbols × native TFs, incremental, paced) and a scheduled `run` (intraday/daily cadences) | — |
| `cli.py` | Typer app: `collect` (one-shot, non-zero exit on errors) and `serve` (uvicorn) | Typer, uvicorn |

Plus `pyproject.toml`, `config.yaml` (reused; detection block added), `tests/`
mirroring the modules, and a `.env.example`.

Each module has one clear responsibility and is independently testable; pure
logic (`indicators`, `detection`, `resample`, `rule`, `timeframes`, `config`)
has no I/O.

## 4. Peak/valley detection (the upgrade)

`detection.py`:

- `smooth(series, period)` — exponential moving average via pandas `ewm(span=period).mean()`; `period == 1` returns the series unchanged. Leading NaN warmup is preserved.
- `find_extrema(series, min_prominence, min_distance)`:
  - peaks: `scipy.signal.find_peaks(values, prominence=min_prominence, distance=min_distance)`
  - valleys: `find_peaks(-values, prominence=min_prominence, distance=min_distance)`
  - NaN warmup is dropped before the call and indices mapped back to the full series.
  - Returns peaks and valleys as lists of `(index, value)`.

The screener smooths each indicator series, runs `find_extrema`, takes the **3
most recent** peaks and valleys, and computes `latest`/`zone`/`trend` on the
**smoothed** series (per the brainstorm decision). `find_peaks` enforces both a
minimum topographic **prominence** (filters noise/insignificant bumps) and a
minimum **distance** in bars (fixes "peaks too close together").

## 5. Configuration

Reuse `config.yaml`. Replace the global `screening.pivot_window` with a
per-indicator `detection` block:

```yaml
screening:
  match: any
  trend_lookback: 3
  peaks_to_show: 3
  peak_lookback: 3mo
  trend_flat_epsilon: 0

indicators:
  rsi:
    length: 14
    source: close
    smoothing: { type: SMA, length: 14, bb_stddev: 2 }   # captured; base RSI used for screening
    detection: { smoothing: 3, min_prominence: 8, min_distance: 5 }    # RSI points
  volume_oscillator:
    short_length: 5
    long_length: 10
    detection: { smoothing: 3, min_prominence: 5, min_distance: 5 }    # %
  distance_from_ma:
    source: close
    ma_type: EMA
    length: 200
    calculation: percent
    detection: { smoothing: 3, min_prominence: 3, min_distance: 5 }    # %
```

Validation (pydantic): `match` ∈ {any, all, min:N≥1}; `trend_lookback≥1`;
`peaks_to_show≥1`; timeframes all known; indicator periods ≥ their minimums;
`detection.smoothing≥1`, `min_prominence≥0`, `min_distance≥1`. The detection
defaults above are starting points and tunable.

## 6. Data flow (unchanged from Go)

- **collect:** for each (symbol, native TF): `last_bar_time` → yfinance fetch
  from there (or full history if none) → drop still-forming bar (closed-bars-only)
  → `upsert_bars`. Per-item errors logged; `collect` exits non-zero if any fail.
  Intraday vs daily timeframes refresh on their own cadences in the scheduled
  `run`.
- **serve (`GET /screen`):** for each (symbol, requested TF): load native bars
  (bounded recent window), resample if derived, compute each indicator, smooth +
  detect, classify zone, compute trend, apply match → rows with full per-indicator
  payload.
- **serve (`GET /matches`):** same engine, then aggregate qualifying rows into
  one lean entry per stock (`symbol`, `timeframes[]`, `indicators[]`); dedupe
  symbols/timeframes; any-timeframe union.
- **`GET /healthz`:** DB ping → 200/503.

## 7. API contract

Identical to the Go service (so existing callers/tests of the JSON shapes still
hold):

- Params (all optional, default from config): `symbols`, `timeframes`, `match`
  (`any|all|min:N`), `indicators`. Unknown symbol/timeframe/indicator or invalid
  match → `400`; duplicate indicator → `400`; symbols/timeframes de-duplicated.
  Engine error → `500` with a generic body (detail logged server-side).
- `/screen` response: `{ as_of, criteria{match,symbols,timeframes}, results[
  {symbol,timeframe,bar_time,price,triggered[],indicators{name:{latest,trend,zone,
  triggered,peaks[{value,time}],valleys[{value,time}]}}} ], warnings[] }`.
- `/matches` response: `{ as_of, criteria, matches[{symbol,timeframes[],
  indicators[]}], warnings[] }`.

Response models are pydantic; FastAPI serializes them and exposes OpenAPI at
`/docs`.

## 8. Storage

Reuse the existing `bars` table (same columns/PK as the Go schema), so already
collected data stays usable:

```sql
CREATE TABLE IF NOT EXISTS bars (
  symbol TEXT NOT NULL, timeframe TEXT NOT NULL, ts TIMESTAMPTZ NOT NULL,
  open DOUBLE PRECISION NOT NULL, high DOUBLE PRECISION NOT NULL,
  low DOUBLE PRECISION NOT NULL, close DOUBLE PRECISION NOT NULL,
  volume DOUBLE PRECISION NOT NULL, PRIMARY KEY (symbol, timeframe, ts)
);
```

`upsert_bars` uses SQLAlchemy's PostgreSQL `insert(...).on_conflict_do_update`.
`get_bars` returns a DataFrame (or list of rows) ascending by ts, bounded to a
recent window (warmup + `peak_lookback`). Engine configured with a bounded pool
(`pool_size`, `max_overflow`, `pool_pre_ping`). DSN built from env with
credentials URL-encoded and `sslmode` from `DB_SSLMODE` (default `require`).

## 9. Testing

pytest, mirroring modules:

- `indicators`: RSI extremes (all-up→100, all-down→0) and a reference value;
  volume oscillator (constant→0); distance (constant→0, known value).
- `detection`: a series with one large peak + a tiny bump → only the large peak
  survives `min_prominence`; two near peaks → only the higher survives
  `min_distance`; valleys symmetric; `smooth` period 1 = identity.
- `resample`: 1h→4h aggregation (OHLCV) and incomplete-trailing-bucket drop.
- `config`: duration parsing (`3mo`), validation rejects bad match / unknown
  timeframe / out-of-range params.
- `rule`: classify zones, qualifies (any/all/min:N incl. min:0 rejected).
- `screener`: end-to-end on a crafted in-memory store/DataFrame → expected rows
  and matches aggregation.
- `api`: FastAPI `TestClient` — `/screen`, `/matches`, `/healthz`; 400s; warnings
  pass-through; full JSON shape.
- `storage`: gated on `SCREENER_TEST_DSN` (or a Postgres testcontainer) —
  upsert/get/last_bar roundtrip. CI runs Postgres via testcontainers
  (`DOCKER_HOST=unix:///Users/sander/.colima/default/docker.sock`,
  `TESTCONTAINERS_RYUK_DISABLED=true`, per the iac onboarding doc).

## 10. Repo transition

Build on branch `rewrite/python-fastapi`; the Python tree **supersedes** the Go
tree on `main` (same supersede pattern used before, via `git merge -s ours` if
needed), with the Go implementation preserved in git history. A PR is opened for
review; merge happens only after approval. Go-specific files (`go.mod`, `go.sum`,
`internal/`, `main.go`, Go `Dockerfile`/compose if any) are removed in the
rewrite; `config.yaml`, `.env.example`, `docker-compose.yaml` (Postgres) and the
`docs/` specs are kept/updated.

## 11. Out of scope (YAGNI)

- Changing the screening rule itself (current vs last-3 peaks/valleys stays;
  tightening with a tolerance band is a separate future change).
- New indicators or endpoints beyond the existing three / three routes.
- Auth/rate-limiting on the API.
- A standalone Dockerfile for the Python service / CI pipeline (can follow once
  the rewrite lands).
