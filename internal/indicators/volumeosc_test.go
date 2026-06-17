package indicators

import (
	"math"
	"testing"
)

func TestVolumeOscConstantIsZero(t *testing.T) {
	vol := []float64{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	got := VolumeOscillator(vol, 5, 10)
	idx := lastNonNaN(got)
	if idx < 0 || !approx(got[idx], 0) {
		t.Errorf("VO of constant volume = %v, want 0", got[idx])
	}
}

func TestVolumeOscWarmup(t *testing.T) {
	vol := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	got := VolumeOscillator(vol, 5, 10)
	// Undefined until the long EMA exists (index long-1 = 9).
	for i := 0; i < 9; i++ {
		if !math.IsNaN(got[i]) {
			t.Errorf("got[%d] = %v, want NaN", i, got[i])
		}
	}
	if math.IsNaN(got[len(got)-1]) {
		t.Error("last value should be defined")
	}
}
