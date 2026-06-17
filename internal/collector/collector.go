package collector

import (
	"context"
	"log"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/datasource/yahoo"
	"github.com/Ruscigno/stock-screener/internal/storage"
	"github.com/Ruscigno/stock-screener/internal/timeframe"
)

type Collector struct {
	store storage.Store
	src   *yahoo.Client
	cfg   *config.Config
}

func New(store storage.Store, src *yahoo.Client, cfg *config.Config) *Collector {
	return &Collector{store: store, src: src, cfg: cfg}
}

// nativeTimeframes reduces the configured timeframes to the distinct native
// ones that must be fetched (derived TFs are computed at query time, but their
// parent native TF must be collected).
func nativeTimeframes(tfs []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, name := range tfs {
		tf, ok := timeframe.Get(name)
		if !ok {
			continue
		}
		if tf.Native {
			add(tf.Name)
		} else {
			add(tf.Parent)
		}
	}
	return out
}

// dropUnclosed removes trailing candles whose bar end (Time + barDur) is after
// now (i.e. still forming).
func dropUnclosed(candles []yahoo.Candle, barDur time.Duration, now time.Time) []yahoo.Candle {
	out := candles
	for len(out) > 0 {
		last := out[len(out)-1]
		if last.Time.Add(barDur).After(now) {
			out = out[:len(out)-1]
		} else {
			break
		}
	}
	return out
}

// CollectOnce fetches every (symbol, native TF) once and upserts the bars.
// Per-item errors are logged and collected, not fatal.
func (c *Collector) CollectOnce(ctx context.Context) []error {
	var errs []error
	natives := nativeTimeframes(c.cfg.Timeframes)
	now := time.Now()
	for _, symbol := range c.cfg.Stocks {
		for _, tfName := range natives {
			tf, _ := timeframe.Get(tfName)
			from, _, err := c.store.LastBarTime(ctx, symbol, tfName)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			candles, err := c.src.Fetch(ctx, symbol, tf.YahooInterval, from)
			if err != nil {
				log.Printf("collect %s %s: %v", symbol, tfName, err)
				errs = append(errs, err)
				continue
			}
			if c.cfg.Collector.UseClosedBarsOnly {
				candles = dropUnclosed(candles, tf.BarDuration, now)
			}
			bars := make([]storage.Bar, 0, len(candles))
			for _, cd := range candles {
				bars = append(bars, storage.Bar{
					Symbol: symbol, Timeframe: tfName, Time: cd.Time,
					Open: cd.Open, High: cd.High, Low: cd.Low, Close: cd.Close, Volume: cd.Volume,
				})
			}
			if err := c.store.UpsertBars(ctx, bars); err != nil {
				errs = append(errs, err)
				continue
			}
			log.Printf("collected %s %s: %d bars", symbol, tfName, len(bars))
			time.Sleep(200 * time.Millisecond) // gentle pacing for Yahoo
		}
	}
	return errs
}

// Run collects on a ticker until ctx is cancelled. Uses the intraday refresh
// cadence as the loop tick (the smaller of the two).
func (c *Collector) Run(ctx context.Context) {
	tick := time.Duration(c.cfg.Collector.Refresh.Intraday)
	if tick <= 0 {
		tick = 15 * time.Minute
	}
	c.CollectOnce(ctx)
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.CollectOnce(ctx)
		}
	}
}
