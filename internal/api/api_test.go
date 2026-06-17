package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
