package collector

import (
	"context"
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/datasource/yahoo"
	"github.com/Ruscigno/stock-screener/internal/storage"
)

func TestNativeTimeframes(t *testing.T) {
	got := nativeTimeframes([]string{"15m", "4h", "1d", "3d", "1h"})
	// derived (4h, 3d) dropped; natives deduped and preserved.
	want := map[string]bool{"15m": true, "1d": true, "1h": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want keys %v", got, want)
	}
	for _, tf := range got {
		if !want[tf] {
			t.Errorf("unexpected native tf %q", tf)
		}
	}
}

func TestDropUnclosed(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	candles := []yahoo.Candle{
		{Time: now.Add(-2 * time.Hour), Close: 1},    // closed (bar end 10:00+1h <= 12:00)
		{Time: now.Add(-30 * time.Minute), Close: 2}, // still forming (ends 12:30 > 12:00)
	}
	out := dropUnclosed(candles, time.Hour, now)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (forming bar dropped)", len(out))
	}
	if out[0].Close != 1 {
		t.Errorf("kept wrong bar: %+v", out[0])
	}
}

func TestSplitByCadence(t *testing.T) {
	intraday, daily := splitByCadence([]string{"15m", "1h", "1d", "1wk", "1mo"})
	wantIntra := map[string]bool{"15m": true, "1h": true}
	wantDaily := map[string]bool{"1d": true, "1wk": true, "1mo": true}
	if len(intraday) != len(wantIntra) {
		t.Fatalf("intraday = %v, want %v", intraday, wantIntra)
	}
	for _, tf := range intraday {
		if !wantIntra[tf] {
			t.Errorf("unexpected intraday tf %q", tf)
		}
	}
	if len(daily) != len(wantDaily) {
		t.Fatalf("daily = %v, want %v", daily, wantDaily)
	}
	for _, tf := range daily {
		if !wantDaily[tf] {
			t.Errorf("unexpected daily tf %q", tf)
		}
	}
}

// --- fakes for collectTimeframes integration ---

type fakeFetcher struct {
	candles  map[string][]yahoo.Candle // keyed by interval
	err      error
	calls    int
	lastFrom time.Time // the `from` watermark of the most recent call
}

func (f *fakeFetcher) Fetch(_ context.Context, _, interval string, from time.Time) ([]yahoo.Candle, error) {
	f.calls++
	f.lastFrom = from
	if f.err != nil {
		return nil, f.err
	}
	return f.candles[interval], nil
}

type memStore struct {
	upserts [][]storage.Bar
}

func (m *memStore) Migrate(context.Context) error { return nil }
func (m *memStore) UpsertBars(_ context.Context, b []storage.Bar) error {
	m.upserts = append(m.upserts, b)
	return nil
}
func (m *memStore) GetBars(context.Context, string, string, int) ([]storage.Bar, error) {
	return nil, nil
}
func (m *memStore) LastBarTime(context.Context, string, string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (m *memStore) Ping(context.Context) error { return nil }
func (m *memStore) Close() error               { return nil }

func testCfg(stocks, tfs []string) *config.Config {
	c := &config.Config{}
	c.Stocks = stocks
	c.Timeframes = tfs
	c.Collector.UseClosedBarsOnly = false
	return c
}

func TestCollectTimeframesUpserts(t *testing.T) {
	fetch := &fakeFetcher{candles: map[string][]yahoo.Candle{
		"1d": {{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Open: 1, High: 2, Low: 0, Close: 1.5, Volume: 10}},
	}}
	store := &memStore{}
	c := New(store, fetch, testCfg([]string{"AAA"}, []string{"1d"}))

	errs := c.CollectOnce(context.Background())
	if len(errs) != 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if len(store.upserts) != 1 || len(store.upserts[0]) != 1 {
		t.Fatalf("upserts = %v, want one batch of one bar", store.upserts)
	}
	if got := store.upserts[0][0]; got.Symbol != "AAA" || got.Timeframe != "1d" || got.Close != 1.5 {
		t.Errorf("bar = %+v", got)
	}
}

func TestCollectTimeframesContinuesOnFetchError(t *testing.T) {
	fetch := &fakeFetcher{err: errTest}
	store := &memStore{}
	// two native TFs; both fetches fail but the loop must continue and collect both errors.
	c := New(store, fetch, testCfg([]string{"AAA"}, []string{"1d", "1h"}))

	errs := c.CollectOnce(context.Background())
	if len(errs) != 2 {
		t.Fatalf("errs = %d, want 2 (one per timeframe, loop continued)", len(errs))
	}
	if len(store.upserts) != 0 {
		t.Errorf("no upserts expected on fetch failure, got %v", store.upserts)
	}
}

var errTest = fmtError("boom")

type fmtError string

func (e fmtError) Error() string { return string(e) }

// watermarkStore reports a fixed non-zero last bar time for every symbol/tf.
type watermarkStore struct {
	memStore
	at time.Time
}

func (w *watermarkStore) LastBarTime(context.Context, string, string) (time.Time, bool, error) {
	return w.at, true, nil
}

func TestWatermarkPassedToFetch(t *testing.T) {
	at := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	fetch := &fakeFetcher{candles: map[string][]yahoo.Candle{}}
	store := &watermarkStore{at: at}
	c := New(store, fetch, testCfg([]string{"AAA"}, []string{"1d"}))

	if errs := c.CollectOnce(context.Background()); len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	if !fetch.lastFrom.Equal(at) {
		t.Errorf("Fetch from = %v, want watermark %v (incremental path not wired)", fetch.lastFrom, at)
	}
}

func TestRunDoesInitialCollectThenStopsOnCtx(t *testing.T) {
	fetch := &fakeFetcher{candles: map[string][]yahoo.Candle{
		"1d": {{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Close: 1}},
	}}
	c := New(&memStore{}, fetch, testCfg([]string{"AAA"}, []string{"1d"}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled: Run should do one initial pass then return

	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancellation")
	}
	if fetch.calls < 1 {
		t.Errorf("expected at least the initial collect to fetch, calls = %d", fetch.calls)
	}
}
