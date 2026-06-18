# stock-screener

Python (FastAPI) service that collects Yahoo Finance OHLCV into Postgres and
screens stocks sitting at a recent extreme (RSI, volume oscillator,
distance-from-MA) across multiple timeframes. Peak/valley detection uses
`scipy.signal.find_peaks` (prominence) on a smoothed series, then keeps the most
recent pivots walking newest→oldest with a >= 30-bar minimum separation between
pivots (the gap to the current bar is unconstrained). It also requires enough
warmed-up history (e.g. the EMA-200 is converged) before emitting any pivots.

## Configure

Edit `config.yaml` (stocks, timeframes, indicator + detection params). Database
credentials and TLS mode come from environment variables (never commit them —
copy `.env.example` to `.env`):

    DB_USER, DB_PASSWORD, DB_HOST, DB_PORT (default 5432), DB_NAME
    DB_SSLMODE (default "require"; use "disable" for the local docker Postgres)

## Run

    python3.12 -m venv .venv && . .venv/bin/activate && pip install -e '.[dev]'
    set -a && . ./.env && set +a              # load DB_* into the environment
    stock-screener collect --config config.yaml   # fetch + store native-timeframe bars
    stock-screener serve   --config config.yaml   # FastAPI on :server.port (OpenAPI at /docs)

## API

- `GET /healthz` — liveness (pings Postgres).
- `GET /screen` — qualifying (stock, timeframe) rows. Optional query params
  `symbols`, `timeframes`, `match` (`any|all|min:N`), `indicators`; each
  defaults to `config.yaml`.
- `GET /matches` — lean per-stock list of what meets the criteria. Same query
  params as `/screen`. Returns one entry per stock:
  `{ "symbol", "timeframes": [...], "indicators": [...] }` — the timeframes where
  it qualified and the union of indicators that triggered (no peaks/valleys; use
  `/screen` for the full detail).

A (stock, timeframe) row qualifies when, per `match`, its indicators are at an
extreme: current value `>=` the lowest of the last 3 peaks (zone `high`) or
`<=` the highest of the last 3 valleys (zone `low`). Each indicator reports its
latest value, trend (rising/falling/flat), and last 3 peaks/valleys.

Timeframes `15m,30m,1h,1d,1wk,1mo` are fetched from Yahoo; `4h` and `3d` are
resampled from `1h`/`1d` at query time. The authoritative list lives in code
(`stock_screener/timeframes.py`); `config.yaml` only selects which are enabled.

## Develop

    pip install -e '.[dev]'
    pytest            # unit tests (storage integration test skips without SCREENER_TEST_DSN)
    ruff check .

See `docs/superpowers/specs/2026-06-18-python-rewrite-design.md` for the design
(and the `2026-06-16`/`2026-06-17` specs for the original requirements).
