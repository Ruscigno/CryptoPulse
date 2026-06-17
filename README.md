# stock-screener

Collects Yahoo Finance OHLCV into Postgres and screens stocks that are sitting
at a recent extreme (RSI, volume oscillator, distance-from-MA) across multiple
timeframes.

## Configure

Edit `config.yaml` (stocks, timeframes, indicator params). Database credentials
and TLS mode come from environment variables (never commit them — copy
`.env.example` to `.env`):

    DB_USER, DB_PASSWORD, DB_HOST, DB_PORT (default 5432), DB_NAME
    DB_SSLMODE (default "require"; use "disable" for the local docker Postgres)

## Run

    docker compose up -d                     # local Postgres (reads creds from .env)
    go run . collect --config config.yaml    # fetch + store native-timeframe bars
    go run . serve   --config config.yaml    # HTTP API (runs collector in-process if enabled)

## API

- `GET /healthz` — liveness (pings Postgres).
- `GET /screen` — qualifying (stock, timeframe) rows. Optional query params
  `symbols`, `timeframes`, `match` (`any|all|min:N`), `indicators`; each
  defaults to `config.yaml`.

A (stock, timeframe) row qualifies when, per `match`, its indicators are at an
extreme: current value `>=` the lowest of the last 3 peaks (zone `high`) or
`<=` the highest of the last 3 valleys (zone `low`). Each indicator reports its
latest value, trend (rising/falling/flat), and last 3 peaks/valleys.

Timeframes `15m,30m,1h,1d,1wk,1mo` are fetched from Yahoo; `4h` and `3d` are
resampled from `1h`/`1d` at query time. The authoritative list lives in code
(`internal/timeframe`); `config.yaml` only selects which are enabled.

See `docs/superpowers/specs/2026-06-16-stock-screener-design.md` for the full design.
