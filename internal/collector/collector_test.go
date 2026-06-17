package collector

import (
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/datasource/yahoo"
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
