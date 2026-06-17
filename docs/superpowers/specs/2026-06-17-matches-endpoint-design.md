# `GET /matches` — lean per-stock screening list — Design

**Date:** 2026-06-17
**Status:** Approved (design)

## 1. Goal

Add a lean endpoint that answers "which stocks meet the criteria right now?" as a
flat per-stock list, without the heavy per-indicator payload (`peaks`/`valleys`/
`trend`/`zone`) that `GET /screen` returns. `/screen` stays as the detailed view;
`/matches` is the everyday "give me the list" view.

## 2. Endpoint

`GET /matches` — same query params as `/screen`, all optional, each defaulting to
`config.yaml`:

| Param | Example | Default |
|---|---|---|
| `symbols` | `AAPL,MSFT` | all config stocks |
| `timeframes` | `1d` | all config timeframes |
| `match` | `any` \| `all` \| `min:2` | config (`any`) |
| `indicators` | `rsi,distance_from_ma` | all three |

Passing `?timeframes=1d` narrows evaluation to that timeframe; with no
`timeframes`, all configured timeframes are evaluated and a stock qualifies if it
meets the criteria on **any** of them.

Validation is identical to `/screen`: unknown symbol/timeframe → `400`; invalid
`match` → `400`; unknown or duplicate indicator → `400`; backing-store failure →
`500` (generic body, detail logged server-side).

## 3. Semantics

`/matches` runs the **same screening engine** as `/screen`
(`screener.Screen(req)`), which yields the qualifying `(stock, timeframe)` rows,
then **aggregates those rows by stock**:

- A stock appears in `matches` iff it has at least one qualifying
  `(stock, timeframe)` row (union across the evaluated timeframes).
- `timeframes` — the timeframes where the stock qualified, in config order.
- `indicators` — the union of indicator names that triggered across those
  timeframes, de-duplicated, in canonical order
  (`rsi`, `volume_oscillator`, `distance_from_ma`).
- `matches` is ordered by the request's symbol order (config order).

The per-(stock, timeframe) qualification itself is unchanged: a row qualifies
when, per `match`, its indicators are at an extreme (the existing rule). The flat
`indicators` list intentionally drops the timeframe↔indicator mapping; callers
who need that detail use `/screen`.

## 4. Response shape

```json
{
  "as_of": "2026-06-17T20:00:00Z",
  "criteria": { "match": "any", "symbols": 5, "timeframes": ["1d", "4h"] },
  "matches": [
    { "symbol": "AAPL", "timeframes": ["1d", "4h"], "indicators": ["rsi", "volume_oscillator", "distance_from_ma"] },
    { "symbol": "MSFT", "timeframes": ["1d"], "indicators": ["rsi"] }
  ],
  "warnings": [
    { "symbol": "TSLA", "timeframe": "15m", "message": "insufficient_data: distance_from_ma needs 200 bars, have 140" }
  ]
}
```

`warnings` is carried through unchanged from the screener result (same shape as
`/screen`). `matches` is empty (`[]`, never null) when nothing qualifies.

## 5. Implementation

All changes live in `internal/api` — the `screener` package is **not** modified
(the endpoint reuses `screener.Screen`).

- **DRY the request handling:** extract the shared param parsing + validation
  currently inside `handleScreen` into a helper
  `parseScreenRequest(r) (screener.Request, *apiError)` used by both `/screen`
  and `/matches`, so the validation rules live in one place.
- **`handleMatches`:** parse+validate via the shared helper → call
  `s.scr.Screen(ctx, req)` → aggregate with a pure function
  `aggregateMatches(res screener.Result, req screener.Request) []matchDTO` →
  encode the response.
- **DTOs:** `matchDTO{ Symbol string; Timeframes []string; Indicators []string }`
  and a `matchesResponseDTO` mirroring the existing `responseDTO` (with `matches`
  instead of `results`).
- Register `/matches` in `Server.Handler()`'s mux.

`aggregateMatches` is a pure function (no I/O), so the dedup/ordering logic is
unit-testable without HTTP or a screener.

## 6. Testing

- **`aggregateMatches` unit test:** feed a `screener.Result` with rows for
  AAPL (1d → rsi+volume_oscillator) and AAPL (4h → rsi) and MSFT (1d →
  distance_from_ma); assert two matches, AAPL with `timeframes=[1d,4h]` and the
  de-duplicated, canonically-ordered `indicators=[rsi,volume_oscillator,
  distance_from_ma]`, MSFT with `[1d]`/`[distance_from_ma]`; assert symbol order
  follows the request.
- **`/matches` handler test (fixture-backed fake screener):** asserts `200`,
  the JSON `matches` shape, and warnings pass-through.
- **Validation reuse test:** `/matches?timeframes=7m` → `400`,
  `/matches?indicators=bogus` → `400` (confirms the shared validation path).

## 7. Out of scope (YAGNI)

- Per-timeframe indicator breakdown in `/matches` (that's what `/screen` is for).
- A `min_timeframes` knob (stock-level aggregation is plain "any timeframe" for
  now; can be added later if needed).
- Sorting/pagination options.
