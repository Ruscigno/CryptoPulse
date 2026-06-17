package indicators

import (
	"math"
	"strings"
)

// DistanceFromMA returns the percentage distance of price from its moving
// average: (close - MA) / MA * 100. maType is "SMA" or "EMA" (default EMA).
func DistanceFromMA(closes []float64, maType string, period int) []float64 {
	var ma []float64
	if strings.EqualFold(maType, "SMA") {
		ma = SMA(closes, period)
	} else {
		ma = EMA(closes, period)
	}
	out := nanSlice(len(closes))
	for i := range closes {
		if math.IsNaN(ma[i]) || ma[i] == 0 {
			continue
		}
		out[i] = (closes[i] - ma[i]) / ma[i] * 100
	}
	return out
}
