package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/screener"
)

type fakeScreener struct{ res screener.Result }

func (f *fakeScreener) Screen(context.Context, screener.Request) (screener.Result, error) {
	return f.res, nil
}

type fakePinger struct{ err error }

func (f *fakePinger) Ping(context.Context) error { return f.err }

func testCfg() *config.Config {
	c := &config.Config{}
	c.Stocks = []string{"AAPL"}
	c.Timeframes = []string{"1d"}
	c.Screening.Match = "any"
	return c
}

func TestScreenEndpoint(t *testing.T) {
	res := screener.Result{
		Rows: []screener.Row{{
			Symbol: "AAPL", Timeframe: "1d", BarTime: time.Now().UTC(), Price: 200,
			Triggered: []string{"rsi"},
			Indicators: map[string]screener.IndicatorResult{
				"rsi": {Latest: 28.3, Trend: "rising", Zone: "low", Triggered: true,
					Peaks: []screener.PivotPoint{{Value: 70, Time: time.Now().UTC()}}},
			},
		}},
	}
	srv := NewServer(&fakeScreener{res: res}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/screen", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Results []struct {
			Symbol    string   `json:"symbol"`
			Timeframe string   `json:"timeframe"`
			Triggered []string `json:"triggered"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Results) != 1 || body.Results[0].Symbol != "AAPL" {
		t.Fatalf("results = %+v", body.Results)
	}
}

func TestHealthz(t *testing.T) {
	srv := NewServer(&fakeScreener{}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200", rec.Code)
	}
}

func TestScreenRejectsUnknownTimeframe(t *testing.T) {
	srv := NewServer(&fakeScreener{}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/screen?timeframes=7m", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestScreenRejectsBadMinMatch(t *testing.T) {
	srv := NewServer(&fakeScreener{}, &fakePinger{}, testCfg())
	for _, m := range []string{"min:0", "min:abc", "min:"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/screen?match="+m, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("match=%q: status = %d, want 400", m, rec.Code)
		}
	}
}

func TestScreenAcceptsValidMin(t *testing.T) {
	srv := NewServer(&fakeScreener{}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/screen?match=min:2", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("match=min:2: status = %d, want 200", rec.Code)
	}
}

func TestScreenRejectsUnknownSymbol(t *testing.T) {
	srv := NewServer(&fakeScreener{}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/screen?symbols=NOTREAL", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for unknown symbol", rec.Code)
	}
}

func TestScreenResponseShapeAndWarnings(t *testing.T) {
	res := screener.Result{
		Rows: []screener.Row{{
			Symbol: "AAPL", Timeframe: "1d",
			BarTime:   time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			Price:     198.42,
			Triggered: []string{"rsi"},
			Indicators: map[string]screener.IndicatorResult{
				"rsi": {
					Latest: 28.3, Trend: "rising", Zone: "low", Triggered: true,
					Peaks:   []screener.PivotPoint{{Value: 72.1, Time: time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)}},
					Valleys: []screener.PivotPoint{{Value: 27.5, Time: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)}},
				},
			},
		}},
		Warnings: []screener.Warning{{Symbol: "TSLA", Timeframe: "1d", Message: "insufficient_data: need 200 bars"}},
	}
	srv := NewServer(&fakeScreener{res: res}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/screen?symbols=AAPL&timeframes=1d", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Criteria struct {
			Match      string   `json:"match"`
			Timeframes []string `json:"timeframes"`
		} `json:"criteria"`
		Results []struct {
			Symbol     string  `json:"symbol"`
			BarTime    string  `json:"bar_time"`
			Price      float64 `json:"price"`
			Indicators map[string]struct {
				Zone  string `json:"zone"`
				Trend string `json:"trend"`
				Peaks []struct {
					Value float64 `json:"value"`
					Time  string  `json:"time"`
				} `json:"peaks"`
				Valleys []struct {
					Value float64 `json:"value"`
				} `json:"valleys"`
			} `json:"indicators"`
		} `json:"results"`
		Warnings []struct {
			Symbol  string `json:"symbol"`
			Message string `json:"message"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Criteria.Match != "any" || len(body.Criteria.Timeframes) != 1 || body.Criteria.Timeframes[0] != "1d" {
		t.Errorf("criteria = %+v", body.Criteria)
	}
	if len(body.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(body.Results))
	}
	r := body.Results[0]
	if r.Price != 198.42 || r.BarTime == "" {
		t.Errorf("row price/bar_time = %v/%q", r.Price, r.BarTime)
	}
	rsi, ok := r.Indicators["rsi"]
	if !ok {
		t.Fatal("missing rsi indicator in JSON")
	}
	if rsi.Zone != "low" || rsi.Trend != "rising" {
		t.Errorf("rsi zone/trend = %q/%q", rsi.Zone, rsi.Trend)
	}
	if len(rsi.Peaks) != 1 || rsi.Peaks[0].Value != 72.1 || rsi.Peaks[0].Time == "" {
		t.Errorf("rsi peaks = %+v", rsi.Peaks)
	}
	if len(rsi.Valleys) != 1 || rsi.Valleys[0].Value != 27.5 {
		t.Errorf("rsi valleys = %+v", rsi.Valleys)
	}
	if len(body.Warnings) != 1 || body.Warnings[0].Symbol != "TSLA" || body.Warnings[0].Message == "" {
		t.Errorf("warnings = %+v", body.Warnings)
	}
}

func TestScreenRejectsUnknownAndDuplicateIndicators(t *testing.T) {
	srv := NewServer(&fakeScreener{}, &fakePinger{}, testCfg())
	for _, q := range []string{"indicators=bogus", "indicators=rsi,rsi"} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/screen?"+q, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rec.Code)
		}
	}
}

// erroringScreener always fails, to exercise the 500 path.
type erroringScreener struct{}

func (erroringScreener) Screen(context.Context, screener.Request) (screener.Result, error) {
	return screener.Result{}, errBoom
}

var errBoom = boomErr("internal detail that must not leak")

type boomErr string

func (e boomErr) Error() string { return string(e) }

func TestScreenInternalErrorIsGeneric(t *testing.T) {
	srv := NewServer(erroringScreener{}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/screen", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "internal detail") {
		t.Errorf("response leaked internal error: %q", rec.Body.String())
	}
}
