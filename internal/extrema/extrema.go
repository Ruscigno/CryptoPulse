package extrema

import "math"

// Pivot is a confirmed local extremum in a series.
type Pivot struct {
	Index int
	Value float64
}

// FindPeaks returns indices that are strictly greater than the w values on
// each side. Windows touching a NaN are skipped.
func FindPeaks(values []float64, w int) []Pivot {
	return find(values, w, true)
}

// FindValleys returns indices that are strictly less than the w values on
// each side. Windows touching a NaN are skipped.
func FindValleys(values []float64, w int) []Pivot {
	return find(values, w, false)
}

func find(values []float64, w int, peak bool) []Pivot {
	var out []Pivot
	for i := w; i < len(values)-w; i++ {
		v := values[i]
		if math.IsNaN(v) {
			continue
		}
		ok := true
		for j := 1; j <= w && ok; j++ {
			l, r := values[i-j], values[i+j]
			if math.IsNaN(l) || math.IsNaN(r) {
				ok = false
				break
			}
			if peak {
				ok = v > l && v > r
			} else {
				ok = v < l && v < r
			}
		}
		if ok {
			out = append(out, Pivot{Index: i, Value: v})
		}
	}
	return out
}

// LastN returns the last n pivots (most recent), or all if n >= len.
func LastN(pivots []Pivot, n int) []Pivot {
	if n >= len(pivots) {
		return pivots
	}
	return pivots[len(pivots)-n:]
}
