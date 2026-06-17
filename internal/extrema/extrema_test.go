package extrema

import (
	"math"
	"testing"
)

func TestFindPeaksAndValleys(t *testing.T) {
	v := []float64{1, 3, 1, 5, 1}
	peaks := FindPeaks(v, 1)
	if len(peaks) != 2 || peaks[0].Index != 1 || peaks[1].Index != 3 {
		t.Fatalf("peaks = %+v, want indices 1 and 3", peaks)
	}
	valleys := FindValleys(v, 1)
	if len(valleys) != 1 || valleys[0].Index != 2 {
		t.Fatalf("valleys = %+v, want index 2", valleys)
	}
}

func TestPivotsIgnoreNaN(t *testing.T) {
	v := []float64{math.NaN(), 3, 1, 5, 1}
	// index 1 cannot be a peak: its left neighbor is NaN.
	peaks := FindPeaks(v, 1)
	if len(peaks) != 1 || peaks[0].Index != 3 {
		t.Fatalf("peaks = %+v, want only index 3", peaks)
	}
}

func TestLastN(t *testing.T) {
	in := []Pivot{{1, 1}, {2, 2}, {3, 3}, {4, 4}}
	got := LastN(in, 2)
	if len(got) != 2 || got[0].Index != 3 || got[1].Index != 4 {
		t.Fatalf("LastN = %+v, want last two", got)
	}
	if len(LastN(in, 10)) != 4 {
		t.Error("LastN with n>len should return all")
	}
}
