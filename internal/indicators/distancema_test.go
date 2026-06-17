package indicators

import "testing"

func TestDistanceFromSMA(t *testing.T) {
	// SMA period 2 of [10,20] -> [NaN,15]; distance at idx1 = (20-15)/15*100.
	got := DistanceFromMA([]float64{10, 20}, "SMA", 2)
	if !approx(got[1], (20-15)/15.0*100) {
		t.Errorf("distance = %v, want %v", got[1], (20-15)/15.0*100)
	}
}

func TestDistanceConstantIsZero(t *testing.T) {
	got := DistanceFromMA([]float64{10, 10, 10, 10, 10}, "EMA", 3)
	idx := lastNonNaN(got)
	if !approx(got[idx], 0) {
		t.Errorf("distance of constant price = %v, want 0", got[idx])
	}
}

func TestDistanceDefaultsToEMA(t *testing.T) {
	// Unknown MA type falls back to EMA without panicking.
	got := DistanceFromMA([]float64{1, 2, 3, 4}, "weird", 2)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
}
