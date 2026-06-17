package screener

import (
	"strconv"
	"strings"

	"github.com/Ruscigno/stock-screener/internal/extrema"
)

// classify returns the zone for the current value given recent pivots:
// "high" if current >= the lowest of the last peaks, "low" if current <= the
// highest of the last valleys, otherwise "neutral". High takes precedence.
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

// qualifies reports whether a row with `triggered` of `requested` indicators
// firing satisfies the match mode. For "all", every requested indicator must
// trigger (so an insufficient-data indicator makes "all" unsatisfiable).
func qualifies(triggered, requested int, match string) bool {
	switch {
	case match == "any":
		return triggered >= 1
	case match == "all":
		return requested > 0 && triggered == requested
	case strings.HasPrefix(match, "min:"):
		n, err := strconv.Atoi(strings.TrimPrefix(match, "min:"))
		if err != nil {
			return false
		}
		return triggered >= n
	}
	return false
}
