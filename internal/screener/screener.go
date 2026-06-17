package screener

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/extrema"
	"github.com/Ruscigno/stock-screener/internal/indicators"
	"github.com/Ruscigno/stock-screener/internal/match"
	"github.com/Ruscigno/stock-screener/internal/resample"
	"github.com/Ruscigno/stock-screener/internal/storage"
	"github.com/Ruscigno/stock-screener/internal/timeframe"
)

// trendEpsilon is the dead-band for the rising/falling/flat slope. It is
// intentionally near-zero: for continuous indicator values an exact tie between
// two bars is vanishingly rare, so trend is effectively always rising/falling —
// which is the directional signal we want. "flat" is reserved for genuine
// equality (e.g. constant/insufficient series). If a wider dead-band is ever
// needed, promote this to a configurable screening setting.
const trendEpsilon = 1e-9

type Screener struct {
	store storage.Store
	cfg   *config.Config
}

func New(store storage.Store, cfg *config.Config) *Screener {
	return &Screener{store: store, cfg: cfg}
}

func (s *Screener) Screen(ctx context.Context, req Request) (Result, error) {
	var res Result
	for _, symbol := range req.Symbols {
		for _, tfName := range req.Timeframes {
			tf, ok := timeframe.Get(tfName)
			if !ok {
				res.Warnings = append(res.Warnings, Warning{symbol, tfName, "unknown timeframe"})
				continue
			}
			bars, err := s.loadBars(ctx, symbol, tf)
			if err != nil {
				res.Warnings = append(res.Warnings, Warning{symbol, tfName, "load error: " + err.Error()})
				continue
			}
			if len(bars) == 0 {
				res.Warnings = append(res.Warnings, Warning{symbol, tfName, "no_data"})
				continue
			}
			row, warns := s.evaluate(symbol, tfName, bars, req)
			res.Warnings = append(res.Warnings, warns...)
			if row != nil {
				res.Rows = append(res.Rows, *row)
			}
		}
	}
	return res, nil
}

func (s *Screener) loadBars(ctx context.Context, symbol string, tf timeframe.TF) ([]storage.Bar, error) {
	need := requiredBars(s.cfg, tf)
	if tf.Native {
		return s.store.GetBars(ctx, symbol, tf.Name, need)
	}
	// Derived TFs aggregate GroupSize parent bars each, so fetch proportionally
	// more parent bars before resampling.
	parent, err := s.store.GetBars(ctx, symbol, tf.Parent, need*tf.GroupSize)
	if err != nil {
		return nil, err
	}
	if s.cfg.Collector.UseClosedBarsOnly {
		return resample.ToClosed(parent, tf.Name), nil
	}
	return resample.To(parent, tf.Name), nil
}

// requiredBars is how many bars (in tf's own resolution) the screener loads per
// (symbol, timeframe). It is the larger of:
//   - the warmup the longest indicator needs plus room to confirm several
//     pivots (a hard floor, so indicators never come back all-NaN), and
//   - the peak_lookback window converted to bars (the configured minimum
//     history to scan for recent pivots).
//
// Bounding the load this way keeps each /screen request O(window) instead of
// O(full stored history).
func requiredBars(cfg *config.Config, tf timeframe.TF) int {
	longest := cfg.Indicators.RSI.Length
	if l := cfg.Indicators.VolumeOscillator.LongLength; l > longest {
		longest = l
	}
	if l := cfg.Indicators.DistanceFromMA.Length; l > longest {
		longest = l
	}
	warmup := longest + (2*cfg.Screening.PivotWindow+1)*(cfg.Screening.PeaksToShow+1) + 50

	lookbackBars := 0
	if tf.BarDuration > 0 {
		lookbackBars = int(time.Duration(cfg.Screening.PeakLookback) / tf.BarDuration)
	}
	if lookbackBars > warmup {
		return lookbackBars
	}
	return warmup
}

func (s *Screener) evaluate(symbol, tfName string, bars []storage.Bar, req Request) (*Row, []Warning) {
	var warns []Warning
	closes := make([]float64, len(bars))
	volumes := make([]float64, len(bars))
	for i, b := range bars {
		closes[i] = b.Close
		volumes[i] = b.Volume
	}
	w := s.cfg.Screening.PivotWindow
	nShow := s.cfg.Screening.PeaksToShow
	results := map[string]IndicatorResult{}
	var triggered []string

	for _, ind := range req.Indicators {
		series := s.series(ind, closes, volumes)
		if series == nil {
			warns = append(warns, Warning{symbol, tfName, "unknown indicator: " + ind})
			continue
		}
		idx := lastNonNaNScreener(series)
		if idx < 0 {
			warns = append(warns, Warning{symbol, tfName,
				fmt.Sprintf("insufficient_data: %s has no value (need more bars)", ind)})
			continue
		}
		peaks := extrema.LastN(extrema.FindPeaks(series, w), nShow)
		valleys := extrema.LastN(extrema.FindValleys(series, w), nShow)
		zone := classify(series[idx], peaks, valleys)
		ir := IndicatorResult{
			Latest:    series[idx],
			Trend:     trend(series, idx, s.cfg.Screening.TrendLookback),
			Zone:      zone,
			Triggered: zone != "neutral",
			Peaks:     toPoints(peaks, bars),
			Valleys:   toPoints(valleys, bars),
		}
		results[ind] = ir
		if ir.Triggered {
			triggered = append(triggered, ind)
		}
	}

	if !match.Qualifies(len(triggered), len(req.Indicators), req.Match) {
		return nil, warns
	}
	last := bars[len(bars)-1]
	return &Row{
		Symbol: symbol, Timeframe: tfName, BarTime: last.Time, Price: last.Close,
		Triggered: triggered, Indicators: results,
	}, warns
}

func (s *Screener) series(ind string, closes, volumes []float64) []float64 {
	switch ind {
	case IndRSI:
		return indicators.RSI(closes, s.cfg.Indicators.RSI.Length)
	case IndVolOsc:
		return indicators.VolumeOscillator(volumes,
			s.cfg.Indicators.VolumeOscillator.ShortLength,
			s.cfg.Indicators.VolumeOscillator.LongLength)
	case IndDistance:
		return indicators.DistanceFromMA(closes,
			s.cfg.Indicators.DistanceFromMA.MAType,
			s.cfg.Indicators.DistanceFromMA.Length)
	}
	return nil
}

func trend(series []float64, idx, lookback int) string {
	prev := idx - lookback
	if prev < 0 || math.IsNaN(series[prev]) {
		return "flat"
	}
	diff := series[idx] - series[prev]
	switch {
	case diff > trendEpsilon:
		return "rising"
	case diff < -trendEpsilon:
		return "falling"
	default:
		return "flat"
	}
}

func toPoints(pivots []extrema.Pivot, bars []storage.Bar) []PivotPoint {
	out := make([]PivotPoint, 0, len(pivots))
	for _, p := range pivots {
		out = append(out, PivotPoint{Value: p.Value, Time: bars[p.Index].Time})
	}
	return out
}

func lastNonNaNScreener(v []float64) int {
	for i := len(v) - 1; i >= 0; i-- {
		if !math.IsNaN(v[i]) {
			return i
		}
	}
	return -1
}
