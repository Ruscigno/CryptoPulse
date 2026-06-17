package screener

import "github.com/Ruscigno/stock-screener/internal/extrema"

// classify returns the zone for the current value given recent pivots:
// "high" if current >= the lowest of the last peaks, "low" if current <= the
// highest of the last valleys, otherwise "neutral".
//
// Precedence: in choppy data the recent peaks and valleys can overlap
// (min(peaks) <= max(valleys)), so a single value may satisfy both conditions.
// We deliberately resolve that ambiguity in favour of "high" — being back at a
// recent peak is the stronger, more actionable signal.
func classify(current float64, peaks, valleys []extrema.Pivot) string {
	if len(peaks) > 0 {
		min := peaks[0].Value
		for _, p := range peaks {
			if p.Value < min {
				min = p.Value
			}
		}
		if current >= min {
			return "high"
		}
	}
	if len(valleys) > 0 {
		max := valleys[0].Value
		for _, v := range valleys {
			if v.Value > max {
				max = v.Value
			}
		}
		if current <= max {
			return "low"
		}
	}
	return "neutral"
}
