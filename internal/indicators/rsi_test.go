package indicators

import (
	"math"
	"testing"
)

func TestRSIAllGains(t *testing.T) {
	// Strictly increasing -> no losses -> RSI = 100.
	closes := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	got := RSI(closes, 14)
	idx := lastNonNaN(got)
	if idx < 0 || !approx(got[idx], 100) {
		t.Errorf("last RSI = %v, want 100", got[idx])
	}
}

func TestRSIAllLosses(t *testing.T) {
	closes := []float64{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	got := RSI(closes, 14)
	idx := lastNonNaN(got)
	if idx < 0 || !approx(got[idx], 0) {
		t.Errorf("last RSI = %v, want 0", got[idx])
	}
}

func TestRSIWarmupAndBounds(t *testing.T) {
	closes := []float64{44, 44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28}
	got := RSI(closes, 14)
	// First valid RSI is at index = period (14); earlier are NaN.
	for i := 0; i < 14; i++ {
		if !math.IsNaN(got[i]) {
			t.Errorf("got[%d] = %v, want NaN", i, got[i])
		}
	}
	for i := 14; i < len(got); i++ {
		if got[i] < 0 || got[i] > 100 {
			t.Errorf("got[%d] = %v out of [0,100]", i, got[i])
		}
	}
}
