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

func TestQualifies(t *testing.T) {
	if !qualifies(1, 3, "any") {
		t.Error("any: 1 trigger should qualify")
	}
	if qualifies(0, 3, "any") {
		t.Error("any: 0 triggers should not qualify")
	}
	if !qualifies(3, 3, "all") {
		t.Error("all: 3/3 should qualify")
	}
	if qualifies(2, 3, "all") {
		t.Error("all: 2/3 should not qualify")
	}
	if !qualifies(2, 3, "min:2") {
		t.Error("min:2: 2 triggers should qualify")
	}
	if qualifies(1, 3, "min:2") {
		t.Error("min:2: 1 trigger should not qualify")
	}
}
