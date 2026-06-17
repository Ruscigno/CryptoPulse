package timeframe

import "time"

// TF describes one timeframe the screener understands.
type TF struct {
	Name          string        // canonical name, e.g. "1h", "4h"
	Native        bool          // true = fetched from Yahoo; false = resampled
	YahooInterval string        // native only: Yahoo's interval string
	Parent        string        // derived only: native TF it is built from
	GroupSize     int           // derived only: parent bars per bucket
	BarDuration   time.Duration // approximate bar length (for lookback->bars hints)
}

var registry = map[string]TF{
	"15m": {Name: "15m", Native: true, YahooInterval: "15m", BarDuration: 15 * time.Minute},
	"30m": {Name: "30m", Native: true, YahooInterval: "30m", BarDuration: 30 * time.Minute},
	"1h":  {Name: "1h", Native: true, YahooInterval: "60m", BarDuration: time.Hour},
	"4h":  {Name: "4h", Native: false, Parent: "1h", GroupSize: 4, BarDuration: 4 * time.Hour},
	"1d":  {Name: "1d", Native: true, YahooInterval: "1d", BarDuration: 24 * time.Hour},
	"3d":  {Name: "3d", Native: false, Parent: "1d", GroupSize: 3, BarDuration: 3 * 24 * time.Hour},
	"1wk": {Name: "1wk", Native: true, YahooInterval: "1wk", BarDuration: 7 * 24 * time.Hour},
	"1mo": {Name: "1mo", Native: true, YahooInterval: "1mo", BarDuration: 30 * 24 * time.Hour},
}

func Get(name string) (TF, bool) {
	tf, ok := registry[name]
	return tf, ok
}

// BucketStart returns the start of the bucket that t falls into for this
// timeframe, anchored to fixed UTC boundaries. For native timeframes it
// truncates to BarDuration; for derived it uses the parent grouping.
func (tf TF) BucketStart(t time.Time) time.Time {
	t = t.UTC()
	switch tf.Name {
	case "3d":
		days := t.Unix() / 86400
		start := days - days%3
		return time.Unix(start*86400, 0).UTC()
	default:
		// 4h and native sub-day TFs align cleanly to BarDuration since the
		// Unix epoch lies on those boundaries.
		return t.Truncate(tf.BarDuration)
	}
}
