# `GET /matches` Endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `GET /matches`, a lean per-stock screening list that reuses the existing screening engine and aggregates qualifying `(stock, timeframe)` rows into one entry per stock (`{symbol, timeframes[], indicators[]}`), keeping the detailed `/screen` untouched.

**Architecture:** All changes are in `internal/api`. The shared query-param parsing/validation is extracted from `handleScreen` into one helper used by both endpoints (DRY). `handleMatches` runs `screener.Screen(req)` (the same engine) and folds the rows by symbol via a pure `aggregateMatches` function. The `screener` package is not modified.

**Tech Stack:** Go 1.23, standard `net/http` + `encoding/json`; existing `internal/screener`, `internal/timeframe`, `internal/match`, `internal/config`.

**Spec:** `docs/superpowers/specs/2026-06-17-matches-endpoint-design.md`

---

## File structure

- `internal/api/api.go` — modify: extract `parseRequest`, add `aggregateMatches`, `matchDTO`/`matchesResponseDTO`/`toMatchesDTO`, `handleMatches`, and the `/matches` route.
- `internal/api/api_test.go` — modify: add unit test for `aggregateMatches` and handler tests for `/matches`.

No new files; the api package stays cohesive and small.

---

## Task 1: Extract shared request parsing/validation

Refactor the param parsing + validation out of `handleScreen` into a reusable
method so `/screen` and `/matches` validate identically.

**Files:**
- Modify: `internal/api/api.go`

- [ ] **Step 1: Add the `reqError` type and `parseRequest` method**

Add near the top of `api.go` (after the `Server` type):

```go
// reqError is a client-facing request error with an HTTP status.
type reqError struct {
	status int
	msg    string
}

// parseRequest builds a screener.Request from the query string (each param
// defaulting to config) and validates it. Shared by /screen and /matches.
func (s *Server) parseRequest(r *http.Request) (screener.Request, *reqError) {
	req := screener.Request{
		Symbols:    csvOrDefault(r.URL.Query().Get("symbols"), s.cfg.Stocks),
		Timeframes: csvOrDefault(r.URL.Query().Get("timeframes"), s.cfg.Timeframes),
		Match:      orDefault(r.URL.Query().Get("match"), s.cfg.Screening.Match),
		Indicators: csvOrDefault(r.URL.Query().Get("indicators"), screener.AllIndicators),
	}
	for _, tf := range req.Timeframes {
		if _, ok := timeframe.Get(tf); !ok {
			return req, &reqError{http.StatusBadRequest, "unknown timeframe: " + tf}
		}
	}
	allowed := make(map[string]bool, len(s.cfg.Stocks))
	for _, sym := range s.cfg.Stocks {
		allowed[sym] = true
	}
	for _, sym := range req.Symbols {
		if !allowed[sym] {
			return req, &reqError{http.StatusBadRequest, "unknown symbol: " + sym}
		}
	}
	if !match.Valid(req.Match) {
		return req, &reqError{http.StatusBadRequest, "invalid match mode: " + req.Match}
	}
	if err := validateIndicators(req.Indicators); err != nil {
		return req, &reqError{http.StatusBadRequest, err.Error()}
	}
	return req, nil
}
```

- [ ] **Step 2: Rewrite `handleScreen` to use `parseRequest`**

Replace the body of `handleScreen` (the inline `req := ...` construction plus the
timeframe loop, symbol loop, `match.Valid` check, and `validateIndicators` call)
with:

```go
func (s *Server) handleScreen(w http.ResponseWriter, r *http.Request) {
	req, rerr := s.parseRequest(r)
	if rerr != nil {
		http.Error(w, rerr.msg, rerr.status)
		return
	}
	result, err := s.scr.Screen(r.Context(), req)
	if err != nil {
		log.Printf("screen failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, toDTO(result, req))
}
```

(`csvOrDefault`, `orDefault`, `validateIndicators`, `toDTO`, `writeJSON` already
exist and are unchanged.)

- [ ] **Step 3: Build and run the existing api tests**

Run: `go build ./... && go test ./internal/api/ -v`
Expected: PASS. The existing tests (`TestScreenEndpoint`, `TestHealthz`,
`TestScreenRejectsUnknownTimeframe`, `TestScreenRejectsUnknownSymbol`,
`TestScreenRejectsBadMinMatch`, `TestScreenAcceptsValidMin`,
`TestScreenRejectsUnknownAndDuplicateIndicators`, `TestScreenInternalErrorIsGeneric`,
`TestScreenResponseShapeAndWarnings`) all still pass — they now exercise the
extracted `parseRequest` path.

- [ ] **Step 4: Commit**

```bash
git add internal/api/api.go
git commit -m "refactor(api): extract shared parseRequest for /screen (reused by /matches)"
```

---

## Task 2: `aggregateMatches` pure function

Fold qualifying rows into one entry per stock. Pure (no I/O), so it is unit-tested
directly.

**Files:**
- Modify: `internal/api/api.go` (add types + function)
- Test: `internal/api/api_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/api/api_test.go`:

```go
func TestAggregateMatches(t *testing.T) {
	res := screener.Result{
		Rows: []screener.Row{
			{Symbol: "AAPL", Timeframe: "1d", Triggered: []string{"rsi", "volume_oscillator"}},
			{Symbol: "AAPL", Timeframe: "4h", Triggered: []string{"rsi"}},
			{Symbol: "MSFT", Timeframe: "1d", Triggered: []string{"distance_from_ma"}},
		},
	}
	req := screener.Request{
		Symbols:    []string{"AAPL", "MSFT", "TSLA"}, // TSLA has no rows -> excluded
		Timeframes: []string{"1d", "4h"},
	}
	got := aggregateMatches(res, req)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// Order follows req.Symbols.
	if got[0].Symbol != "AAPL" || got[1].Symbol != "MSFT" {
		t.Fatalf("order = %s,%s, want AAPL,MSFT", got[0].Symbol, got[1].Symbol)
	}
	// AAPL: timeframes in req order; indicators deduped in canonical order.
	if len(got[0].Timeframes) != 2 || got[0].Timeframes[0] != "1d" || got[0].Timeframes[1] != "4h" {
		t.Errorf("AAPL timeframes = %v, want [1d 4h]", got[0].Timeframes)
	}
	wantInds := []string{"rsi", "volume_oscillator", "distance_from_ma"}
	// distance_from_ma did NOT trigger for AAPL, so it must be absent.
	if len(got[0].Indicators) != 2 || got[0].Indicators[0] != "rsi" || got[0].Indicators[1] != "volume_oscillator" {
		t.Errorf("AAPL indicators = %v, want [rsi volume_oscillator]", got[0].Indicators)
	}
	if len(got[1].Indicators) != 1 || got[1].Indicators[0] != "distance_from_ma" {
		t.Errorf("MSFT indicators = %v, want [distance_from_ma]", got[1].Indicators)
	}
	_ = wantInds
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestAggregateMatches -v`
Expected: FAIL — `aggregateMatches` / `matchDTO` undefined.

- [ ] **Step 3: Add the DTO and the function**

Add to `internal/api/api.go` (near the other DTOs):

```go
type matchDTO struct {
	Symbol     string   `json:"symbol"`
	Timeframes []string `json:"timeframes"`
	Indicators []string `json:"indicators"`
}

// aggregateMatches folds qualifying (stock, timeframe) rows into one entry per
// stock: the timeframes where it qualified (in request order) and the union of
// indicators that triggered (deduped, canonical order). Stocks with no
// qualifying row are omitted. Result order follows req.Symbols.
func aggregateMatches(res screener.Result, req screener.Request) []matchDTO {
	type agg struct {
		tfSeen  map[string]bool
		indSeen map[string]bool
	}
	bySym := make(map[string]*agg, len(req.Symbols))
	for _, row := range res.Rows {
		a := bySym[row.Symbol]
		if a == nil {
			a = &agg{tfSeen: map[string]bool{}, indSeen: map[string]bool{}}
			bySym[row.Symbol] = a
		}
		a.tfSeen[row.Timeframe] = true
		for _, ind := range row.Triggered {
			a.indSeen[ind] = true
		}
	}

	out := make([]matchDTO, 0, len(bySym))
	for _, sym := range req.Symbols {
		a := bySym[sym]
		if a == nil {
			continue
		}
		tfs := make([]string, 0, len(a.tfSeen))
		for _, tf := range req.Timeframes {
			if a.tfSeen[tf] {
				tfs = append(tfs, tf)
			}
		}
		inds := make([]string, 0, len(a.indSeen))
		for _, name := range screener.AllIndicators {
			if a.indSeen[name] {
				inds = append(inds, name)
			}
		}
		out = append(out, matchDTO{Symbol: sym, Timeframes: tfs, Indicators: inds})
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -run TestAggregateMatches -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go
git commit -m "feat(api): aggregateMatches folds screening rows into per-stock matches"
```

---

## Task 3: `/matches` endpoint (handler, response DTO, route)

**Files:**
- Modify: `internal/api/api.go` (response DTO, `toMatchesDTO`, `handleMatches`, route)
- Test: `internal/api/api_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/api_test.go`:

```go
func TestMatchesEndpoint(t *testing.T) {
	res := screener.Result{
		Rows: []screener.Row{
			{Symbol: "AAPL", Timeframe: "1d", Triggered: []string{"rsi", "volume_oscillator"}},
			{Symbol: "AAPL", Timeframe: "4h", Triggered: []string{"rsi"}},
		},
		Warnings: []screener.Warning{{Symbol: "MSFT", Timeframe: "1d", Message: "no_data"}},
	}
	srv := NewServer(&fakeScreener{res: res}, &fakePinger{}, matchesTestCfg())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/matches", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Criteria struct {
			Match string `json:"match"`
		} `json:"criteria"`
		Matches []struct {
			Symbol     string   `json:"symbol"`
			Timeframes []string `json:"timeframes"`
			Indicators []string `json:"indicators"`
		} `json:"matches"`
		Warnings []struct {
			Message string `json:"message"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Matches) != 1 || body.Matches[0].Symbol != "AAPL" {
		t.Fatalf("matches = %+v", body.Matches)
	}
	m := body.Matches[0]
	if len(m.Timeframes) != 2 || m.Timeframes[0] != "1d" || m.Timeframes[1] != "4h" {
		t.Errorf("timeframes = %v", m.Timeframes)
	}
	if len(m.Indicators) != 2 || m.Indicators[0] != "rsi" || m.Indicators[1] != "volume_oscillator" {
		t.Errorf("indicators = %v", m.Indicators)
	}
	if len(body.Warnings) != 1 || body.Warnings[0].Message != "no_data" {
		t.Errorf("warnings = %+v", body.Warnings)
	}
	if body.Criteria.Match != "any" {
		t.Errorf("criteria.match = %q, want any", body.Criteria.Match)
	}
}

func TestMatchesValidatesParams(t *testing.T) {
	srv := NewServer(&fakeScreener{}, &fakePinger{}, matchesTestCfg())
	for _, q := range []string{"timeframes=7m", "indicators=bogus", "symbols=NOPE", "match=min:0"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/matches?"+q, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rec.Code)
		}
	}
}

func matchesTestCfg() *config.Config {
	c := &config.Config{}
	c.Stocks = []string{"AAPL"}
	c.Timeframes = []string{"1d", "4h"}
	c.Screening.Match = "any"
	return c
}
```

(`fakeScreener`, `fakePinger` already exist in `api_test.go`. Note: `testCfg()`
already exists with `Timeframes=["1d"]`; this plan uses a separate
`matchesTestCfg()` with `["1d","4h"]` so the 4h row is allowed.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run TestMatches -v`
Expected: FAIL — `/matches` route not registered (404) / `toMatchesDTO` undefined.

- [ ] **Step 3: Add the response DTO, `toMatchesDTO`, handler, and route**

Add to `internal/api/api.go`:

```go
type matchesResponseDTO struct {
	AsOf     time.Time `json:"as_of"`
	Criteria struct {
		Match      string   `json:"match"`
		Symbols    int      `json:"symbols"`
		Timeframes []string `json:"timeframes"`
	} `json:"criteria"`
	Matches  []matchDTO   `json:"matches"`
	Warnings []warningDTO `json:"warnings"`
}

func toMatchesDTO(res screener.Result, req screener.Request) matchesResponseDTO {
	var out matchesResponseDTO
	out.AsOf = time.Now().UTC()
	out.Criteria.Match = req.Match
	out.Criteria.Symbols = len(req.Symbols)
	out.Criteria.Timeframes = req.Timeframes
	out.Matches = aggregateMatches(res, req)
	out.Warnings = make([]warningDTO, 0, len(res.Warnings))
	for _, wn := range res.Warnings {
		out.Warnings = append(out.Warnings, warningDTO(wn))
	}
	return out
}

func (s *Server) handleMatches(w http.ResponseWriter, r *http.Request) {
	req, rerr := s.parseRequest(r)
	if rerr != nil {
		http.Error(w, rerr.msg, rerr.status)
		return
	}
	result, err := s.scr.Screen(r.Context(), req)
	if err != nil {
		log.Printf("matches failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, toMatchesDTO(result, req))
}
```

Register the route in `Handler()` — change it to:

```go
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/screen", s.handleScreen)
	mux.HandleFunc("/matches", s.handleMatches)
	mux.HandleFunc("/healthz", s.handleHealthz)
	return mux
}
```

(`warningDTO` already exists and is reused; `aggregateMatches`/`matchDTO` come
from Task 2.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/ -v`
Expected: PASS (all api tests, old and new).

- [ ] **Step 5: Full build/vet/test**

Run: `go build ./... && go vet ./... && gofmt -l . && go test ./...`
Expected: build/vet clean; `gofmt -l .` prints nothing; all packages pass.

- [ ] **Step 6: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go
git commit -m "feat(api): add GET /matches lean per-stock screening list"
```

---

## Task 4: Document the endpoint

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add `/matches` to the API section of README.md**

Under the `GET /screen` bullet in the `## API` section, add:

```markdown
- `GET /matches` — lean per-stock list of what meets the criteria. Same query
  params as `/screen`. Returns one entry per stock:
  `{ "symbol", "timeframes": [...], "indicators": [...] }` — the timeframes where
  it qualified and the union of indicators that triggered (no peaks/valleys; use
  `/screen` for the full detail).
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: document GET /matches in README"
```

---

## Self-Review Notes (completed during planning)

- **Spec coverage:** endpoint + params/validation (Task 1 `parseRequest` + Task 3 route/tests); per-stock aggregation with any-timeframe union, config-order timeframes, canonical deduped indicators (Task 2 `aggregateMatches`); response shape incl. `criteria`/`matches`/`warnings` (Task 3 `toMatchesDTO`); reuse of `screener.Screen` with no screener changes (Tasks 3); tests for aggregation, handler shape, warnings pass-through, and validation reuse (Tasks 2–3). README doc (Task 4). All spec sections map to a task.
- **Type consistency:** `matchDTO` defined in Task 2 and consumed by `matchesResponseDTO`/`toMatchesDTO` in Task 3; `parseRequest`/`reqError` defined in Task 1 and used by `handleScreen` (Task 1) and `handleMatches` (Task 3); `aggregateMatches(res, req)` signature consistent across tasks; reuses existing `warningDTO`, `csvOrDefault`, `orDefault`, `validateIndicators`, `writeJSON`, `screener.AllIndicators`.
- **No placeholders:** every code step is complete and compiles against the current `api.go`.
