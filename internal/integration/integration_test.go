// Package integration holds end-to-end tests that wire the real Postgres store
// together with the collector and screener. The market-data source is faked
// (no network) so the test is deterministic; only the DB is real. Gated on
// SCREENER_TEST_DSN so it is skipped unless a throwaway Postgres is provided.
package integration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/Ruscigno/stock-screener/internal/collector"
	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/datasource/yahoo"
	"github.com/Ruscigno/stock-screener/internal/screener"
	"github.com/Ruscigno/stock-screener/internal/storage"
)

const testSymbol = "ITEST"

// fakeFetcher returns a fixed daily series, ignoring symbol/interval/from.
type fakeFetcher struct{ candles []yahoo.Candle }

func (f *fakeFetcher) Fetch(context.Context, string, string, time.Time) ([]yahoo.Candle, error) {
	return f.candles, nil
}

func testCfg() *config.Config {
	c := &config.Config{}
	c.Stocks = []string{testSymbol}
	c.Timeframes = []string{"1d"}
	c.Collector.UseClosedBarsOnly = false
	c.Screening.Match = "any"
	c.Screening.PivotWindow = 1
	c.Screening.TrendLookback = 1
	c.Screening.PeaksToShow = 3
	c.Indicators.RSI.Length = 14
	c.Indicators.VolumeOscillator.ShortLength = 5
	c.Indicators.VolumeOscillator.LongLength = 10
	c.Indicators.DistanceFromMA.MAType = "SMA"
	c.Indicators.DistanceFromMA.Length = 3
	return c
}

func TestCollectThenScreen(t *testing.T) {
	dsn := os.Getenv("SCREENER_TEST_DSN")
	if dsn == "" {
		t.Skip("set SCREENER_TEST_DSN to run the end-to-end integration test")
	}
	ctx := context.Background()

	store, err := storage.NewPostgresStore(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanup(t, dsn)
	defer cleanup(t, dsn)

	// V-shaped closes so distance-from-SMA(3) ends at a low -> distance triggers.
	closes := []float64{10, 12, 14, 12, 10, 12, 14, 16, 14, 12, 10}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []yahoo.Candle
	for i, c := range closes {
		candles = append(candles, yahoo.Candle{
			Time: t0.AddDate(0, 0, i), Open: c, High: c, Low: c, Close: c, Volume: 100,
		})
	}

	cfg := testCfg()
	col := collector.New(store, &fakeFetcher{candles: candles}, cfg)
	if errs := col.CollectOnce(ctx); len(errs) != 0 {
		t.Fatalf("collect errors: %v", errs)
	}

	// Re-running collection must be idempotent (upsert, no duplicates).
	if errs := col.CollectOnce(ctx); len(errs) != 0 {
		t.Fatalf("second collect errors: %v", errs)
	}
	bars, err := store.GetBars(ctx, testSymbol, "1d", 0)
	if err != nil {
		t.Fatalf("get bars: %v", err)
	}
	if len(bars) != len(closes) {
		t.Fatalf("stored bars = %d, want %d (idempotent upsert)", len(bars), len(closes))
	}

	scr := screener.New(store, cfg)
	res, err := scr.Screen(ctx, screener.Request{
		Symbols: []string{testSymbol}, Timeframes: []string{"1d"},
		Match: "any", Indicators: []string{screener.IndDistance},
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1 (distance at a low)", len(res.Rows))
	}
	if res.Rows[0].Symbol != testSymbol || res.Rows[0].Timeframe != "1d" {
		t.Errorf("row = %+v", res.Rows[0])
	}
	if ind, ok := res.Rows[0].Indicators[screener.IndDistance]; !ok || ind.Zone != "low" {
		t.Errorf("distance indicator = %+v (want zone low)", ind)
	}
}

func cleanup(t *testing.T, dsn string) {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("cleanup connect: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("DELETE FROM bars WHERE symbol = $1", testSymbol); err != nil {
		t.Fatalf("cleanup delete: %v", err)
	}
}
