package screener

import (
	"testing"

	"github.com/Ruscigno/stock-screener/internal/extrema"
)

func TestClassify(t *testing.T) {
	peaks := []extrema.Pivot{{Index: 1, Value: 70}, {Index: 5, Value: 60}}   // min peak = 60
	valleys := []extrema.Pivot{{Index: 2, Value: 30}, {Index: 6, Value: 40}} // max valley = 40
	if z := classify(65, peaks, valleys); z != "high" {
		t.Errorf("classify(65) = %q, want high (>= min peak 60)", z)
	}
	if z := classify(35, peaks, valleys); z != "low" {
		t.Errorf("classify(35) = %q, want low (<= max valley 40)", z)
	}
	if z := classify(50, peaks, valleys); z != "neutral" {
		t.Errorf("classify(50) = %q, want neutral", z)
	}
}

func TestClassifyOverlapPrefersHigh(t *testing.T) {
	// Overlapping pivots (min peak 40 <= max valley 60): a value of 50 satisfies
	// both >= min(peaks) and <= max(valleys); precedence resolves to "high".
	peaks := []extrema.Pivot{{Index: 1, Value: 40}}
	valleys := []extrema.Pivot{{Index: 2, Value: 60}}
	if z := classify(50, peaks, valleys); z != "high" {
		t.Errorf("classify(50) = %q, want high (precedence)", z)
	}
}

func TestTrend(t *testing.T) {
	rising := []float64{1, 2, 3}
	if got := trend(rising, 2, 1, 0); got != "rising" {
		t.Errorf("rising: got %q", got)
	}
	falling := []float64{3, 2, 1}
	if got := trend(falling, 2, 1, 0); got != "falling" {
		t.Errorf("falling: got %q", got)
	}
	// Exact tie is flat with eps 0.
	flat := []float64{5, 5}
	if got := trend(flat, 1, 1, 0); got != "flat" {
		t.Errorf("flat(eps0): got %q", got)
	}
	// Small change becomes flat when within a positive epsilon.
	small := []float64{100.0, 100.05}
	if got := trend(small, 1, 1, 0.1); got != "flat" {
		t.Errorf("flat(eps0.1): got %q", got)
	}
	if got := trend(small, 1, 1, 0.0); got != "rising" {
		t.Errorf("rising(eps0): got %q", got)
	}
}
