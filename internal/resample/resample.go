package resample

import (
	"github.com/Ruscigno/stock-screener/internal/storage"
	"github.com/Ruscigno/stock-screener/internal/timeframe"
)

// To aggregates native bars into the target derived timeframe. Input must be
// ascending by time. Buckets are anchored to fixed UTC boundaries.
func To(bars []storage.Bar, targetTF string) []storage.Bar {
	tf, ok := timeframe.Get(targetTF)
	if !ok || tf.Native || len(bars) == 0 {
		return nil
	}
	var out []storage.Bar
	var cur *storage.Bar
	curStart := tf.BucketStart(bars[0].Time)
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
		}
	}
	for _, b := range bars {
		start := tf.BucketStart(b.Time)
		if cur == nil || !start.Equal(curStart) {
			flush()
			curStart = start
			nb := storage.Bar{Symbol: b.Symbol, Timeframe: targetTF, Time: start,
				Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume}
			cur = &nb
			continue
		}
		if b.High > cur.High {
			cur.High = b.High
		}
		if b.Low < cur.Low {
			cur.Low = b.Low
		}
		cur.Close = b.Close
		cur.Volume += b.Volume
	}
	flush()
	return out
}

// ToClosed is To but drops the final bucket if it has fewer than GroupSize
// parent bars (i.e. the bucket is still forming).
func ToClosed(bars []storage.Bar, targetTF string) []storage.Bar {
	tf, ok := timeframe.Get(targetTF)
	if !ok || tf.Native {
		return nil
	}
	full := To(bars, targetTF)
	if len(full) == 0 {
		return full
	}
	lastStart := tf.BucketStart(bars[len(bars)-1].Time)
	n := 0
	for i := len(bars) - 1; i >= 0; i-- {
		if tf.BucketStart(bars[i].Time).Equal(lastStart) {
			n++
		} else {
			break
		}
	}
	if n < tf.GroupSize {
		return full[:len(full)-1]
	}
	return full
}
