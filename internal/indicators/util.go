package indicators

import "math"

func nanSlice(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	return out
}

// lastNonNaN returns the index of the last non-NaN value, or -1 if none.
func lastNonNaN(v []float64) int {
	for i := len(v) - 1; i >= 0; i-- {
		if !math.IsNaN(v[i]) {
			return i
		}
	}
	return -1
}
