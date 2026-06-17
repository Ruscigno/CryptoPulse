package indicators

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSMA(t *testing.T) {
	got := SMA([]float64{1, 2, 3, 4}, 2)
	want := []float64{math.NaN(), 1.5, 2.5, 3.5}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	if !math.IsNaN(got[0]) {
		t.Errorf("got[0] = %v, want NaN", got[0])
	}
	for i := 1; i < len(want); i++ {
		if !approx(got[i], want[i]) {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestEMA(t *testing.T) {
	got := EMA([]float64{1, 2, 3}, 2) // seed = (1+2)/2 = 1.5; alpha = 2/3
	if !math.IsNaN(got[0]) {
		t.Errorf("got[0] = %v, want NaN", got[0])
	}
	if !approx(got[1], 1.5) {
		t.Errorf("got[1] = %v, want 1.5", got[1])
	}
	if !approx(got[2], 2.5) { // 2/3*3 + 1/3*1.5 = 2.5
		t.Errorf("got[2] = %v, want 2.5", got[2])
	}
}

func TestShortInput(t *testing.T) {
	got := SMA([]float64{1}, 2)
	if len(got) != 1 || !math.IsNaN(got[0]) {
		t.Errorf("got = %v, want [NaN]", got)
	}
}

func TestLastNonNaN(t *testing.T) {
	idx := lastNonNaN([]float64{math.NaN(), 1, 2, math.NaN()})
	if idx != 2 {
		t.Errorf("lastNonNaN = %d, want 2", idx)
	}
	if lastNonNaN([]float64{math.NaN()}) != -1 {
		t.Error("all-NaN should return -1")
	}
}
