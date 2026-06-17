package screener

import "time"

// Request parameters for one screen call (already defaulted from config).
type Request struct {
	Symbols    []string
	Timeframes []string
	Match      string
	Indicators []string
}

// PivotPoint is a peak/valley exposed in the result.
type PivotPoint struct {
	Value float64
	Time  time.Time
}

// IndicatorResult is one indicator's evaluation for a (symbol, timeframe).
type IndicatorResult struct {
	Latest    float64
	Trend     string // rising | falling | flat
	Zone      string // high | low | neutral
	Triggered bool
	Peaks     []PivotPoint
	Valleys   []PivotPoint
}

// Row is a qualifying (symbol, timeframe) with all evaluated indicators.
type Row struct {
	Symbol     string
	Timeframe  string
	BarTime    time.Time
	Price      float64
	Triggered  []string
	Indicators map[string]IndicatorResult
}

type Warning struct {
	Symbol    string
	Timeframe string
	Message   string
}

type Result struct {
	Rows     []Row
	Warnings []Warning
}

// Indicator name constants.
const (
	IndRSI      = "rsi"
	IndVolOsc   = "volume_oscillator"
	IndDistance = "distance_from_ma"
)

// AllIndicators is the default evaluation set.
var AllIndicators = []string{IndRSI, IndVolOsc, IndDistance}
