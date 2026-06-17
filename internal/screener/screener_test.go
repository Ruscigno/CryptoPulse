package screener

import (
	"context"
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/storage"
)

// fakeStore returns canned bars for one (symbol, timeframe).
type fakeStore struct{ bars []storage.Bar }

func (f *fakeStore) Migrate(context.Context) error                   { return nil }
func (f *fakeStore) UpsertBars(context.Context, []storage.Bar) error { return nil }
func (f *fakeStore) GetBars(_ context.Context, _, _ string, _ int) ([]storage.Bar, error) {
	return f.bars, nil
}
func (f *fakeStore) LastBarTime(context.Context, string, string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close() error               { return nil }

func testConfig() *config.Config {
	c := &config.Config{}
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

// buildBars makes a daily series from closes (volume constant).
func buildBars(closes []float64) []storage.Bar {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var bars []storage.Bar
	for i, c := range closes {
		bars = append(bars, storage.Bar{
			Symbol: "AAA", Timeframe: "1d", Time: t0.AddDate(0, 0, i),
			Open: c, High: c, Low: c, Close: c, Volume: 100,
		})
	}
	return bars
}

func TestScreenDistanceLowTriggers(t *testing.T) {
	// Distance-from-SMA(3): a V-shape so the latest close sits at/below a
	// recent distance valley -> "low" zone -> triggers under match=any.
	closes := []float64{10, 12, 14, 12, 10, 12, 14, 16, 14, 12, 10}
	s := New(&fakeStore{bars: buildBars(closes)}, testConfig())
	res, err := s.Screen(context.Background(), Request{
		Symbols: []string{"AAA"}, Timeframes: []string{"1d"},
		Match: "any", Indicators: []string{IndDistance},
	})
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	r := res.Rows[0]
	if r.Symbol != "AAA" || r.Timeframe != "1d" {
		t.Errorf("row meta = %+v", r)
	}
	if _, ok := r.Indicators[IndDistance]; !ok {
		t.Error("missing distance_from_ma indicator")
	}
}

func TestScreenInsufficientDataWarns(t *testing.T) {
	s := New(&fakeStore{bars: buildBars([]float64{10, 11, 12})}, testConfig())
	res, err := s.Screen(context.Background(), Request{
		Symbols: []string{"AAA"}, Timeframes: []string{"1d"},
		Match: "any", Indicators: []string{IndRSI},
	})
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected an insufficient_data warning for RSI")
	}
}
