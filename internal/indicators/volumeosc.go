package indicators

import "math"

// VolumeOscillator returns 100*(EMA_short(vol) - EMA_long(vol)) / EMA_long(vol).
// Output is NaN until both EMAs are defined.
func VolumeOscillator(volumes []float64, short, long int) []float64 {
	out := nanSlice(len(volumes))
	emaShort := EMA(volumes, short)
	emaLong := EMA(volumes, long)
	for i := range volumes {
		if math.IsNaN(emaShort[i]) || math.IsNaN(emaLong[i]) || emaLong[i] == 0 {
			continue
		}
		out[i] = (emaShort[i] - emaLong[i]) / emaLong[i] * 100
	}
	return out
}
