package match

import "testing"

func TestValid(t *testing.T) {
	valid := []string{"any", "all", "min:1", "min:2", "min:10"}
	for _, m := range valid {
		if !Valid(m) {
			t.Errorf("Valid(%q) = false, want true", m)
		}
	}
	invalid := []string{"", "sometimes", "min:0", "min:-1", "min:abc", "min:", "MIN:2", "Any"}
	for _, m := range invalid {
		if Valid(m) {
			t.Errorf("Valid(%q) = true, want false", m)
		}
	}
}

func TestQualifies(t *testing.T) {
	cases := []struct {
		triggered, requested int
		mode                 string
		want                 bool
	}{
		{1, 3, "any", true},
		{0, 3, "any", false},
		{3, 3, "all", true},
		{2, 3, "all", false},
		{0, 0, "all", false}, // vacuous: nothing requested cannot satisfy "all"
		{2, 3, "min:2", true},
		{1, 3, "min:2", false},
		{5, 3, "min:0", false},   // non-positive N never qualifies
		{5, 3, "min:abc", false}, // malformed never qualifies
		{5, 3, "bogus", false},
	}
	for _, c := range cases {
		if got := Qualifies(c.triggered, c.requested, c.mode); got != c.want {
			t.Errorf("Qualifies(%d,%d,%q) = %v, want %v", c.triggered, c.requested, c.mode, got, c.want)
		}
	}
}
