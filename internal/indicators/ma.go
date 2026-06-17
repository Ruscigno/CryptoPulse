package indicators

// SMA returns the simple moving average. out[i] is the mean of the `period`
// values ending at i; positions before period-1 are NaN.
func SMA(values []float64, period int) []float64 {
	out := nanSlice(len(values))
	if period < 1 || len(values) < period {
		return out
	}
	var sum float64
	for i := 0; i < len(values); i++ {
		sum += values[i]
		if i >= period {
			sum -= values[i-period]
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		}
	}
	return out
}

// EMA returns the exponential moving average. It is seeded at index period-1
// with the SMA of the first `period` values; earlier positions are NaN.
func EMA(values []float64, period int) []float64 {
	out := nanSlice(len(values))
	if period < 1 || len(values) < period {
		return out
	}
	var seed float64
	for i := 0; i < period; i++ {
		seed += values[i]
	}
	seed /= float64(period)
	out[period-1] = seed
	alpha := 2.0 / float64(period+1)
	for i := period; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}
