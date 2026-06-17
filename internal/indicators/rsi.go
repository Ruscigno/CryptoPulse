package indicators

// RSI returns Wilder's Relative Strength Index over close prices. The first
// valid value is at index = period; earlier positions are NaN. When average
// loss is zero, RSI is 100.
func RSI(closes []float64, period int) []float64 {
	out := nanSlice(len(closes))
	if period < 1 || len(closes) < period+1 {
		return out
	}
	var gain, loss float64
	for i := 1; i <= period; i++ {
		ch := closes[i] - closes[i-1]
		if ch > 0 {
			gain += ch
		} else {
			loss -= ch
		}
	}
	avgGain := gain / float64(period)
	avgLoss := loss / float64(period)
	out[period] = rsiFrom(avgGain, avgLoss)
	for i := period + 1; i < len(closes); i++ {
		ch := closes[i] - closes[i-1]
		g, l := 0.0, 0.0
		if ch > 0 {
			g = ch
		} else {
			l = -ch
		}
		avgGain = (avgGain*float64(period-1) + g) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + l) / float64(period)
		out[i] = rsiFrom(avgGain, avgLoss)
	}
	return out
}

func rsiFrom(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}
