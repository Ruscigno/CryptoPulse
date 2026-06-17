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

// collectTimeframes fetches every (symbol, tfName) for the given TF names and
// upserts the bars. Per-item errors are logged and collected, not fatal.
func (c *Collector) collectTimeframes(ctx context.Context, tfNames []string) []error {
	var errs []error
	now := time.Now()
	for _, symbol := range c.cfg.Stocks {
		for _, tfName := range tfNames {
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

// CollectOnce fetches every (symbol, native TF) once and upserts the bars.
// Per-item errors are logged and collected, not fatal.
func (c *Collector) CollectOnce(ctx context.Context) []error {
	return c.collectTimeframes(ctx, nativeTimeframes(c.cfg.Timeframes))
}

// splitByCadence partitions native timeframes into intraday (bar < 24h) and
// daily (bar >= 24h) groups, preserving order.
func splitByCadence(tfNames []string) (intraday, daily []string) {
	for _, name := range tfNames {
		tf, ok := timeframe.Get(name)
		if !ok {
			continue
		}
		if tf.BarDuration < 24*time.Hour {
			intraday = append(intraday, name)
		} else {
			daily = append(daily, name)
		}
	}
	return intraday, daily
}

// Run collects on two cadences until ctx is cancelled: intraday timeframes on
// Refresh.Intraday, daily-and-longer timeframes on Refresh.Daily. It does one
// full pass immediately on start.
func (c *Collector) Run(ctx context.Context) {
	natives := nativeTimeframes(c.cfg.Timeframes)
	intraday, daily := splitByCadence(natives)

	intradayTick := time.Duration(c.cfg.Collector.Refresh.Intraday)
	if intradayTick <= 0 {
		intradayTick = 15 * time.Minute
	}
	dailyTick := time.Duration(c.cfg.Collector.Refresh.Daily)
	if dailyTick <= 0 {
		dailyTick = 6 * time.Hour
	}

	c.collectTimeframes(ctx, natives) // initial full pass

	it := time.NewTicker(intradayTick)
	defer it.Stop()
	dt := time.NewTicker(dailyTick)
	defer dt.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-it.C:
			if len(intraday) > 0 {
				c.collectTimeframes(ctx, intraday)
			}
		case <-dt.C:
			if len(daily) > 0 {
				c.collectTimeframes(ctx, daily)
			}
		}
	}
}
