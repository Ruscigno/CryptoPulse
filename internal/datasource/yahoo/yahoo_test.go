package yahoo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseChart(t *testing.T) {
	raw, err := os.ReadFile("testdata/aapl_1d.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	candles, err := parseChart(raw)
	if err != nil {
		t.Fatalf("parseChart: %v", err)
	}
	// The middle row with null open is dropped -> 2 valid candles.
	if len(candles) != 2 {
		t.Fatalf("len = %d, want 2 (null row skipped)", len(candles))
	}
	if candles[0].Close != 105.0 || candles[0].Volume != 1000 {
		t.Errorf("candle0 = %+v", candles[0])
	}
	if candles[1].Close != 106.0 {
		t.Errorf("candle1 close = %v, want 106", candles[1].Close)
	}
	if candles[0].Time.Unix() != 1718236800 {
		t.Errorf("candle0 time = %v", candles[0].Time.Unix())
	}
}

func TestParseChartError(t *testing.T) {
	_, err := parseChart([]byte(`{"chart":{"result":null,"error":{"description":"Not Found"}}}`))
	if err == nil {
		t.Fatal("expected error for empty result")
	}
}

func testClient(server *httptest.Server) *Client {
	c := New()
	c.baseURL = server.URL + "/"
	c.baseDelay = time.Millisecond // keep tests fast
	return c
}

func TestFetchRetriesTransientThenSucceeds(t *testing.T) {
	fixture, err := os.ReadFile("testdata/aapl_1d.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 { // fail the first two attempts with a 503
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	candles, err := testClient(srv).Fetch(context.Background(), "AAPL", "1d", time.Time{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3 (2 retries then success)", got)
	}
	if len(candles) != 2 {
		t.Errorf("candles = %d, want 2", len(candles))
	}
}

func TestFetchDoesNotRetryClientError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound) // 404 is not retryable
	}))
	defer srv.Close()

	if _, err := testClient(srv).Fetch(context.Background(), "AAPL", "1d", time.Time{}); err == nil {
		t.Fatal("expected error for 404")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 4xx)", got)
	}
}

func TestFetchExhaustsRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway) // always 502
	}))
	defer srv.Close()

	if _, err := testClient(srv).Fetch(context.Background(), "AAPL", "1d", time.Time{}); err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3 (maxAttempts)", got)
	}
}

func TestFetchQueryParamsByInterval(t *testing.T) {
	fixture, _ := os.ReadFile("testdata/aapl_1d.json")
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()
	c := testClient(srv)

	// Daily: must use period1/period2 (NOT range=max, which Yahoo coarsens to quarterly).
	if _, err := c.Fetch(context.Background(), "AAPL", "1d", time.Time{}); err != nil {
		t.Fatalf("Fetch 1d: %v", err)
	}
	if gotQuery.Get("range") != "" {
		t.Errorf("1d: range=%q, want empty (period-based)", gotQuery.Get("range"))
	}
	if gotQuery.Get("period1") != "0" || gotQuery.Get("period2") == "" {
		t.Errorf("1d: want period1=0 & period2 set, got period1=%q period2=%q", gotQuery.Get("period1"), gotQuery.Get("period2"))
	}

	// Intraday: range-based.
	if _, err := c.Fetch(context.Background(), "AAPL", "15m", time.Time{}); err != nil {
		t.Fatalf("Fetch 15m: %v", err)
	}
	if gotQuery.Get("range") != "60d" {
		t.Errorf("15m: range=%q, want 60d", gotQuery.Get("range"))
	}

	// Incremental (from set): period1 = from, no range.
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := c.Fetch(context.Background(), "AAPL", "1d", from); err != nil {
		t.Fatalf("Fetch incremental: %v", err)
	}
	if gotQuery.Get("range") != "" || gotQuery.Get("period1") != "1767225600" {
		t.Errorf("incremental: want period1=%d & no range, got period1=%q range=%q", from.Unix(), gotQuery.Get("period1"), gotQuery.Get("range"))
	}
}
