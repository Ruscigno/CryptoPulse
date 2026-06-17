package screener

import (
	"context"
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/storage"
	"github.com/Ruscigno/stock-screener/internal/timeframe"
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

// build1hBarsByBucket makes 4 one-hour bars per 4h bucket, all sharing the
// bucket's close value, starting at a 4h boundary. The resampled 4h close
// series therefore equals bucketCloses.
func build1hBarsByBucket(bucketCloses []float64) []storage.Bar {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // 4h-aligned
	var bars []storage.Bar
	for k, c := range bucketCloses {
		for j := 0; j < 4; j++ {
			ts := t0.Add(time.Duration(k*4+j) * time.Hour)
			bars = append(bars, storage.Bar{
				Symbol: "AAA", Timeframe: "1h", Time: ts,
				Open: c, High: c, Low: c, Close: c, Volume: 100,
			})
		}
	}
	return bars
}

func TestScreenDerivedTimeframe(t *testing.T) {
	// V-shaped 4h closes so distance-from-SMA(3) ends at a low -> triggers.
	// fakeStore returns these 1h bars for the parent GetBars("1h") call;
	// the screener must resample to 4h before evaluating.
	bucketCloses := []float64{10, 12, 14, 12, 10, 12, 14, 16, 14, 12, 10}
	s := New(&fakeStore{bars: build1hBarsByBucket(bucketCloses)}, testConfig())
	res, err := s.Screen(context.Background(), Request{
		Symbols: []string{"AAA"}, Timeframes: []string{"4h"},
		Match: "any", Indicators: []string{IndDistance},
	})
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1 (derived 4h path)", len(res.Rows))
	}
	row := res.Rows[0]
	if row.Timeframe != "4h" {
		t.Errorf("timeframe = %q, want 4h", row.Timeframe)
	}
	// BarTime must be a 4h-aligned resampled bar time, not a raw 1h time.
	if row.BarTime.Hour()%4 != 0 || row.BarTime.Minute() != 0 {
		t.Errorf("bar_time %v is not 4h-aligned (resample not applied?)", row.BarTime)
	}
}

func TestScreenMatchModes(t *testing.T) {
	// Distance triggers (1 indicator). match=all with 1 requested -> qualifies.
	closes := []float64{10, 12, 14, 12, 10, 12, 14, 16, 14, 12, 10}
	s := New(&fakeStore{bars: buildBars(closes)}, testConfig())

	all, err := s.Screen(context.Background(), Request{
		Symbols: []string{"AAA"}, Timeframes: []string{"1d"},
		Match: "all", Indicators: []string{IndDistance},
	})
	if err != nil {
		t.Fatalf("Screen(all): %v", err)
	}
	if len(all.Rows) != 1 {
		t.Errorf("match=all, 1 indicator triggers: rows = %d, want 1", len(all.Rows))
	}

	// min:2 with only 1 indicator requested -> cannot qualify.
	min2, err := s.Screen(context.Background(), Request{
		Symbols: []string{"AAA"}, Timeframes: []string{"1d"},
		Match: "min:2", Indicators: []string{IndDistance},
	})
	if err != nil {
		t.Fatalf("Screen(min:2): %v", err)
	}
	if len(min2.Rows) != 0 {
		t.Errorf("match=min:2 with 1 indicator: rows = %d, want 0", len(min2.Rows))
	}
}

func TestRequiredBars(t *testing.T) {
	cfg := &config.Config{}
	cfg.Indicators.RSI.Length = 14
	cfg.Indicators.VolumeOscillator.LongLength = 10
	cfg.Indicators.DistanceFromMA.Length = 200
	cfg.Screening.PivotWindow = 3
	cfg.Screening.PeaksToShow = 3

	day, _ := timeframe.Get("1d")
	// peak_lookback 0 -> the warmup floor dominates: 200 + (2*3+1)*(3+1) + 50.
	warmup := 200 + (2*3+1)*(3+1) + 50
	if got := requiredBars(cfg, day); got != warmup {
		t.Errorf("requiredBars (lookback 0) = %d, want %d", got, warmup)
	}
	// A 10-year peak_lookback on daily bars dominates the floor.
	cfg.Screening.PeakLookback = config.Duration(3650 * 24 * time.Hour)
	if got := requiredBars(cfg, day); got != 3650 {
		t.Errorf("requiredBars (10y lookback) = %d, want 3650", got)
	}
}

// limitRecordingStore records the limit/timeframe of the last GetBars call.
type limitRecordingStore struct {
	fakeStore
	lastLimit int
	lastTF    string
}

func (r *limitRecordingStore) GetBars(_ context.Context, _, tf string, limit int) ([]storage.Bar, error) {
	r.lastLimit = limit
	r.lastTF = tf
	return r.bars, nil
}

func TestLoadBarsBoundsAndScalesDerived(t *testing.T) {
	cfg := testConfig()
	// Native: store is asked for exactly requiredBars (bounded, non-zero).
	natStore := &limitRecordingStore{fakeStore: fakeStore{bars: buildBars([]float64{1, 2, 3})}}
	s := New(natStore, cfg)
	_, _ = s.Screen(context.Background(), Request{
		Symbols: []string{"AAA"}, Timeframes: []string{"1d"}, Match: "any",
		Indicators: []string{IndDistance},
	})
	day, _ := timeframe.Get("1d")
	if natStore.lastTF != "1d" || natStore.lastLimit != requiredBars(cfg, day) {
		t.Errorf("native load: tf=%q limit=%d, want 1d/%d", natStore.lastTF, natStore.lastLimit, requiredBars(cfg, day))
	}
	if natStore.lastLimit == 0 {
		t.Error("native load must be bounded (non-zero limit), got 0 (full history)")
	}

	// Derived 4h: parent "1h" is fetched with GroupSize x the derived need.
	derStore := &limitRecordingStore{fakeStore: fakeStore{bars: build1hBarsByBucket([]float64{1, 2, 3, 4})}}
	s2 := New(derStore, cfg)
	_, _ = s2.Screen(context.Background(), Request{
		Symbols: []string{"AAA"}, Timeframes: []string{"4h"}, Match: "any",
		Indicators: []string{IndDistance},
	})
	four, _ := timeframe.Get("4h")
	wantLimit := requiredBars(cfg, four) * four.GroupSize
	if derStore.lastTF != "1h" || derStore.lastLimit != wantLimit {
		t.Errorf("derived load: tf=%q limit=%d, want 1h/%d", derStore.lastTF, derStore.lastLimit, wantLimit)
	}
}
