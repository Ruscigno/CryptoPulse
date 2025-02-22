package utils

// nullToZero handles null values in Yahoo's response (sometimes returned as 0 or omitted)
func NullToZero(val float64) float64 {
	if val == 0 || val != val { // NaN check
		return 0
	}
	return val
}
