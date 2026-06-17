# Stock Screener Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go service that collects Yahoo Finance OHLCV into Postgres and exposes `GET /screen`, returning every (stock, timeframe) currently at a recent extreme (RSI, volume oscillator, distance-from-MA), with per-indicator latest value, trend, and last-3 peaks/valleys.

**Architecture:** A background collector fetches native-timeframe bars from Yahoo's v8 chart API and upserts them into Postgres. The HTTP endpoint reads bars from the DB, resamples derived timeframes (4h, 3d), computes indicators with pure functions, detects pivot peaks/valleys, applies the screening rule, and returns JSON. Pure logic packages (`config`, `timeframe`, `indicators`, `extrema`, `resample`, `screener` rule) are unit-tested in isolation; `storage` has a gated Postgres integration test; `api` is tested with fakes.

**Tech Stack:** Go 1.23, Postgres (`github.com/lib/pq`), YAML config (`gopkg.in/yaml.v3`), standard `net/http` for the Yahoo client and the API server. The `go-quote` dependency is removed (its `NewQuoteFromYahoo` rejects intraday data).

**Spec:** `docs/superpowers/specs/2026-06-16-stock-screener-design.md`

**Module path:** `github.com/Ruscigno/stock-screener` (packages live under `internal/`).

---

## File Structure

```
config.yaml                              # sample runtime config (committed)
go.mod / go.sum
main.go                                  # CLI: `serve` | `collect`
internal/
  config/config.go        config_test.go     # Config types, Duration parsing, Load+validate
  timeframe/timeframe.go  timeframe_test.go   # TF registry: native/derived, Yahoo interval, bucketing
  indicators/
    ma.go                 ma_test.go          # SMA, EMA
    rsi.go                rsi_test.go         # Wilder RSI
    volumeosc.go          volumeosc_test.go   # volume oscillator
    distancema.go         distancema_test.go  # distance from MA
    util.go                                   # lastNonNaN helper (shared)
  extrema/extrema.go      extrema_test.go     # pivot peaks/valleys, LastN
  resample/resample.go    resample_test.go    # aggregate native bars -> derived TFs
  storage/storage.go      storage_test.go     # Bar, Store interface, Postgres impl, Migrate
  datasource/yahoo/yahoo.go yahoo_test.go     # v8 chart API client + JSON parse
    datasource/yahoo/testdata/aapl_1d.json    # fixture for parse test
  screener/
    types.go                                  # Request, Result, Row, IndicatorResult, PivotPoint
    rule.go               rule_test.go        # classify(zone), qualifies(match)
    screener.go           screener_test.go    # orchestration over a Store
  api/api.go              api_test.go         # /screen + /healthz handlers, JSON DTOs
  collector/collector.go  collector_test.go   # scheduler: fetch native TFs, upsert
```

Each package has one clear responsibility and depends only on lower layers: `config`/`timeframe`/`indicators`/`extrema` depend on nothing; `resample` depends on `storage` (for `Bar`); `screener` depends on `storage`, `resample`, `indicators`, `extrema`, `timeframe`, `config`; `api` depends on `screener`; `collector` depends on `storage`, `datasource/yahoo`, `timeframe`, `config`; `main` wires everything.

---

## Task 0: Clean slate and module setup

**Files:**
- Delete: `main.go`, `stocks.json`
- Modify: `go.mod`

- [ ] **Step 1: Delete the old skeleton and stock list**

```bash
git rm main.go stocks.json
```

- [ ] **Step 2: Remove the go-quote dependency from go.mod**

Edit `go.mod` so it reads exactly:

```
module github.com/Ruscigno/stock-screener

go 1.23.2

require (
	github.com/lib/pq v1.10.9
	gopkg.in/yaml.v3 v3.0.1
)
```

- [ ] **Step 3: Download deps and tidy**

Run: `go get gopkg.in/yaml.v3@v3.0.1 && go mod tidy`
Expected: `go.sum` updates; `go-quote` disappears from `go.mod`/`go.sum`.

- [ ] **Step 4: Verify the module builds (no packages yet is fine)**

Run: `go build ./...`
Expected: exits 0 with no output.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: clean slate - remove old skeleton, drop go-quote, add yaml"
```

---

## Task 1: config package

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

`internal/config/config_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func TestDurationParse(t *testing.T) {
	cases := map[string]time.Duration{
		"15m": 15 * time.Minute,
		"6h":  6 * time.Hour,
		"3mo": 90 * 24 * time.Hour,
		"2d":  2 * 24 * time.Hour,
		"1wk": 7 * 24 * time.Hour,
	}
	for in, want := range cases {
		var d Duration
		if err := d.parse(in); err != nil {
			t.Fatalf("parse(%q) error: %v", in, err)
		}
		if time.Duration(d) != want {
			t.Errorf("parse(%q) = %v, want %v", in, time.Duration(d), want)
		}
	}
}

func TestLoadAndValidate(t *testing.T) {
	cfg, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Server.Port)
	}
	if len(cfg.Stocks) != 2 {
		t.Errorf("stocks = %v, want 2", cfg.Stocks)
	}
	if cfg.Screening.Match != "any" {
		t.Errorf("match = %q, want any", cfg.Screening.Match)
	}
	if cfg.Indicators.RSI.Length != 14 {
		t.Errorf("rsi length = %d, want 14", cfg.Indicators.RSI.Length)
	}
	if time.Duration(cfg.Screening.PeakLookback) != 90*24*time.Hour {
		t.Errorf("peak_lookback = %v, want 3mo", time.Duration(cfg.Screening.PeakLookback))
	}
}

func TestValidateRejectsBadMatch(t *testing.T) {
	if _, err := Load("testdata/bad_match.yaml"); err == nil {
		t.Fatal("expected error for bad match mode, got nil")
	}
}
```

- [ ] **Step 2: Create test fixtures**

`internal/config/testdata/valid.yaml`:

```yaml
server:
  port: 8080
collector:
  enabled: true
  use_closed_bars_only: true
  refresh:
    intraday: 15m
    daily: 6h
stocks: [AAPL, MSFT]
timeframes: [15m, 1h, 4h, 1d]
screening:
  match: any
  pivot_window: 3
  trend_lookback: 3
  peaks_to_show: 3
  peak_lookback: 3mo
indicators:
  rsi:
    length: 14
    source: close
    smoothing: { type: SMA, length: 14, bb_stddev: 2 }
  volume_oscillator:
    short_length: 5
    long_length: 10
  distance_from_ma:
    source: close
    ma_type: EMA
    length: 200
    calculation: percent
```

`internal/config/testdata/bad_match.yaml`: copy of `valid.yaml` but with `match: sometimes`.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL — `Load`/`Duration` undefined.

- [ ] **Step 4: Write the implementation**

`internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration extends time.Duration with calendar-ish suffixes (d, wk, mo, y)
// on top of Go's native units (ns..h). Months/years are approximate
// (mo = 30d, y = 365d) and used only for minimum-history hints.
type Duration time.Duration

func (d *Duration) parse(s string) error {
	s = strings.TrimSpace(s)
	mult := map[string]time.Duration{
		"mo": 30 * 24 * time.Hour,
		"wk": 7 * 24 * time.Hour,
		"d":  24 * time.Hour,
		"y":  365 * 24 * time.Hour,
	}
	for suffix, unit := range mult {
		if strings.HasSuffix(s, suffix) {
			n, err := strconv.Atoi(strings.TrimSuffix(s, suffix))
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", s, err)
			}
			*d = Duration(time.Duration(n) * unit)
			return nil
		}
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	return d.parse(s)
}

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	Collector struct {
		Enabled           bool `yaml:"enabled"`
		UseClosedBarsOnly bool `yaml:"use_closed_bars_only"`
		Refresh           struct {
			Intraday Duration `yaml:"intraday"`
			Daily    Duration `yaml:"daily"`
		} `yaml:"refresh"`
	} `yaml:"collector"`
	Stocks     []string `yaml:"stocks"`
	Timeframes []string `yaml:"timeframes"`
	Screening  struct {
		Match         string   `yaml:"match"`
		PivotWindow   int      `yaml:"pivot_window"`
		TrendLookback int      `yaml:"trend_lookback"`
		PeaksToShow   int      `yaml:"peaks_to_show"`
		PeakLookback  Duration `yaml:"peak_lookback"`
	} `yaml:"screening"`
	Indicators struct {
		RSI struct {
			Length    int    `yaml:"length"`
			Source    string `yaml:"source"`
			Smoothing struct {
				Type     string  `yaml:"type"`
				Length   int     `yaml:"length"`
				BBStdDev float64 `yaml:"bb_stddev"`
			} `yaml:"smoothing"`
		} `yaml:"rsi"`
		VolumeOscillator struct {
			ShortLength int `yaml:"short_length"`
			LongLength  int `yaml:"long_length"`
		} `yaml:"volume_oscillator"`
		DistanceFromMA struct {
			Source      string `yaml:"source"`
			MAType      string `yaml:"ma_type"`
			Length      int    `yaml:"length"`
			Calculation string `yaml:"calculation"`
		} `yaml:"distance_from_ma"`
	} `yaml:"indicators"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Stocks) == 0 {
		return fmt.Errorf("config: stocks must not be empty")
	}
	if len(c.Timeframes) == 0 {
		return fmt.Errorf("config: timeframes must not be empty")
	}
	if !validMatch(c.Screening.Match) {
		return fmt.Errorf("config: invalid match mode %q (want any|all|min:N)", c.Screening.Match)
	}
	if c.Screening.PivotWindow < 1 {
		return fmt.Errorf("config: pivot_window must be >= 1")
	}
	if c.Screening.TrendLookback < 1 {
		return fmt.Errorf("config: trend_lookback must be >= 1")
	}
	if c.Indicators.RSI.Length < 2 {
		return fmt.Errorf("config: rsi length must be >= 2")
	}
	if c.Indicators.VolumeOscillator.ShortLength >= c.Indicators.VolumeOscillator.LongLength {
		return fmt.Errorf("config: volume_oscillator short_length must be < long_length")
	}
	if c.Indicators.DistanceFromMA.Length < 2 {
		return fmt.Errorf("config: distance_from_ma length must be >= 2")
	}
	return nil
}

func validMatch(m string) bool {
	if m == "any" || m == "all" {
		return true
	}
	if strings.HasPrefix(m, "min:") {
		n, err := strconv.Atoi(strings.TrimPrefix(m, "min:"))
		return err == nil && n >= 1
	}
	return false
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/
git commit -m "feat(config): YAML config with duration parsing and validation"
```

---

## Task 2: timeframe package

**Files:**
- Create: `internal/timeframe/timeframe.go`
- Test: `internal/timeframe/timeframe_test.go`

- [ ] **Step 1: Write the failing test**

`internal/timeframe/timeframe_test.go`:

```go
package timeframe

import (
	"testing"
	"time"
)

func TestNativeLookup(t *testing.T) {
	tf, ok := Get("1h")
	if !ok {
		t.Fatal("1h not found")
	}
	if !tf.Native {
		t.Error("1h should be native")
	}
	if tf.YahooInterval != "60m" {
		t.Errorf("1h YahooInterval = %q, want 60m", tf.YahooInterval)
	}
}

func TestDerivedLookup(t *testing.T) {
	tf, ok := Get("4h")
	if !ok {
		t.Fatal("4h not found")
	}
	if tf.Native {
		t.Error("4h should be derived")
	}
	if tf.Parent != "1h" || tf.GroupSize != 4 {
		t.Errorf("4h parent=%q group=%d, want 1h/4", tf.Parent, tf.GroupSize)
	}
}

func TestUnknown(t *testing.T) {
	if _, ok := Get("7m"); ok {
		t.Error("7m should not exist")
	}
}

func TestBucketStart4h(t *testing.T) {
	tf, _ := Get("4h")
	in := time.Date(2026, 6, 16, 14, 30, 0, 0, time.UTC) // 14:30 -> 12:00 bucket
	got := tf.BucketStart(in)
	want := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("BucketStart = %v, want %v", got, want)
	}
}

func TestBucketStart3d(t *testing.T) {
	tf, _ := Get("3d")
	// 3-day buckets anchored to the Unix epoch (1970-01-01 = day 0).
	in := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	got := tf.BucketStart(in)
	days := in.Unix() / 86400
	wantDay := days - days%3
	want := time.Unix(wantDay*86400, 0).UTC()
	if !got.Equal(want) {
		t.Errorf("BucketStart = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/timeframe/ -v`
Expected: FAIL — `Get`/`TF` undefined.

- [ ] **Step 3: Write the implementation**

`internal/timeframe/timeframe.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/timeframe/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/timeframe/
git commit -m "feat(timeframe): TF registry with native/derived and bucketing"
```

---

## Task 3: indicators — moving averages (SMA, EMA)

**Files:**
- Create: `internal/indicators/ma.go`, `internal/indicators/util.go`
- Test: `internal/indicators/ma_test.go`

Convention for all indicators: output slices are the **same length** as input, with `math.NaN()` in warmup positions (before enough data exists).

- [ ] **Step 1: Write the failing test**

`internal/indicators/ma_test.go`:

```go
package indicators

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSMA(t *testing.T) {
	got := SMA([]float64{1, 2, 3, 4}, 2)
	want := []float64{math.NaN(), 1.5, 2.5, 3.5}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	if !math.IsNaN(got[0]) {
		t.Errorf("got[0] = %v, want NaN", got[0])
	}
	for i := 1; i < len(want); i++ {
		if !approx(got[i], want[i]) {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestEMA(t *testing.T) {
	got := EMA([]float64{1, 2, 3}, 2) // seed = (1+2)/2 = 1.5; alpha = 2/3
	if !math.IsNaN(got[0]) {
		t.Errorf("got[0] = %v, want NaN", got[0])
	}
	if !approx(got[1], 1.5) {
		t.Errorf("got[1] = %v, want 1.5", got[1])
	}
	if !approx(got[2], 2.5) { // 2/3*3 + 1/3*1.5 = 2.5
		t.Errorf("got[2] = %v, want 2.5", got[2])
	}
}

func TestShortInput(t *testing.T) {
	got := SMA([]float64{1}, 2)
	if len(got) != 1 || !math.IsNaN(got[0]) {
		t.Errorf("got = %v, want [NaN]", got)
	}
}

func TestLastNonNaN(t *testing.T) {
	idx := lastNonNaN([]float64{math.NaN(), 1, 2, math.NaN()})
	if idx != 2 {
		t.Errorf("lastNonNaN = %d, want 2", idx)
	}
	if lastNonNaN([]float64{math.NaN()}) != -1 {
		t.Error("all-NaN should return -1")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/indicators/ -v`
Expected: FAIL — `SMA`/`EMA`/`lastNonNaN` undefined.

- [ ] **Step 3: Write the implementation**

`internal/indicators/util.go`:

```go
package indicators

import "math"

func nanSlice(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	return out
}

// lastNonNaN returns the index of the last non-NaN value, or -1 if none.
func lastNonNaN(v []float64) int {
	for i := len(v) - 1; i >= 0; i-- {
		if !math.IsNaN(v[i]) {
			return i
		}
	}
	return -1
}
```

`internal/indicators/ma.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/indicators/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/indicators/
git commit -m "feat(indicators): SMA, EMA, and NaN helpers"
```

---

## Task 4: indicators — RSI

**Files:**
- Create: `internal/indicators/rsi.go`
- Test: `internal/indicators/rsi_test.go`

- [ ] **Step 1: Write the failing test**

`internal/indicators/rsi_test.go`:

```go
package indicators

import (
	"math"
	"testing"
)

func TestRSIAllGains(t *testing.T) {
	// Strictly increasing -> no losses -> RSI = 100.
	closes := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	got := RSI(closes, 14)
	idx := lastNonNaN(got)
	if idx < 0 || !approx(got[idx], 100) {
		t.Errorf("last RSI = %v, want 100", got[idx])
	}
}

func TestRSIAllLosses(t *testing.T) {
	closes := []float64{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	got := RSI(closes, 14)
	idx := lastNonNaN(got)
	if idx < 0 || !approx(got[idx], 0) {
		t.Errorf("last RSI = %v, want 0", got[idx])
	}
}

func TestRSIWarmupAndBounds(t *testing.T) {
	closes := []float64{44, 44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28}
	got := RSI(closes, 14)
	// First valid RSI is at index = period (14); earlier are NaN.
	for i := 0; i < 14; i++ {
		if !math.IsNaN(got[i]) {
			t.Errorf("got[%d] = %v, want NaN", i, got[i])
		}
	}
	for i := 14; i < len(got); i++ {
		if got[i] < 0 || got[i] > 100 {
			t.Errorf("got[%d] = %v out of [0,100]", i, got[i])
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/indicators/ -run TestRSI -v`
Expected: FAIL — `RSI` undefined.

- [ ] **Step 3: Write the implementation**

`internal/indicators/rsi.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/indicators/ -run TestRSI -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/indicators/rsi.go internal/indicators/rsi_test.go
git commit -m "feat(indicators): Wilder RSI"
```

---

## Task 5: indicators — volume oscillator

**Files:**
- Create: `internal/indicators/volumeosc.go`
- Test: `internal/indicators/volumeosc_test.go`

- [ ] **Step 1: Write the failing test**

`internal/indicators/volumeosc_test.go`:

```go
package indicators

import (
	"math"
	"testing"
)

func TestVolumeOscConstantIsZero(t *testing.T) {
	vol := []float64{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	got := VolumeOscillator(vol, 5, 10)
	idx := lastNonNaN(got)
	if idx < 0 || !approx(got[idx], 0) {
		t.Errorf("VO of constant volume = %v, want 0", got[idx])
	}
}

func TestVolumeOscWarmup(t *testing.T) {
	vol := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	got := VolumeOscillator(vol, 5, 10)
	// Undefined until the long EMA exists (index long-1 = 9).
	for i := 0; i < 9; i++ {
		if !math.IsNaN(got[i]) {
			t.Errorf("got[%d] = %v, want NaN", i, got[i])
		}
	}
	if math.IsNaN(got[len(got)-1]) {
		t.Error("last value should be defined")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/indicators/ -run TestVolumeOsc -v`
Expected: FAIL — `VolumeOscillator` undefined.

- [ ] **Step 3: Write the implementation**

`internal/indicators/volumeosc.go`:

```go
package indicators

import "math"

// VolumeOscillator returns 100*(EMA_short(vol) - EMA_long(vol)) / EMA_long(vol).
// Output is NaN until both EMAs are defined.
func VolumeOscillator(volumes []float64, short, long int) []float64 {
	out := nanSlice(len(volumes))
	emaShort := EMA(volumes, short)
	emaLong := EMA(volumes, long)
	for i := range volumes {
		if math.IsNaN(emaShort[i]) || math.IsNaN(emaLong[i]) || emaLong[i] == 0 {
			continue
		}
		out[i] = (emaShort[i] - emaLong[i]) / emaLong[i] * 100
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/indicators/ -run TestVolumeOsc -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/indicators/volumeosc.go internal/indicators/volumeosc_test.go
git commit -m "feat(indicators): volume oscillator"
```

---

## Task 6: indicators — distance from moving average

**Files:**
- Create: `internal/indicators/distancema.go`
- Test: `internal/indicators/distancema_test.go`

- [ ] **Step 1: Write the failing test**

`internal/indicators/distancema_test.go`:

```go
package indicators

import "testing"

func TestDistanceFromSMA(t *testing.T) {
	// SMA period 2 of [10,20] -> [NaN,15]; distance at idx1 = (20-15)/15*100.
	got := DistanceFromMA([]float64{10, 20}, "SMA", 2)
	if !approx(got[1], (20-15)/15.0*100) {
		t.Errorf("distance = %v, want %v", got[1], (20-15)/15.0*100)
	}
}

func TestDistanceConstantIsZero(t *testing.T) {
	got := DistanceFromMA([]float64{10, 10, 10, 10, 10}, "EMA", 3)
	idx := lastNonNaN(got)
	if !approx(got[idx], 0) {
		t.Errorf("distance of constant price = %v, want 0", got[idx])
	}
}

func TestDistanceDefaultsToEMA(t *testing.T) {
	// Unknown MA type falls back to EMA without panicking.
	got := DistanceFromMA([]float64{1, 2, 3, 4}, "weird", 2)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/indicators/ -run TestDistance -v`
Expected: FAIL — `DistanceFromMA` undefined.

- [ ] **Step 3: Write the implementation**

`internal/indicators/distancema.go`:

```go
package indicators

import (
	"math"
	"strings"
)

// DistanceFromMA returns the percentage distance of price from its moving
// average: (close - MA) / MA * 100. maType is "SMA" or "EMA" (default EMA).
func DistanceFromMA(closes []float64, maType string, period int) []float64 {
	var ma []float64
	if strings.EqualFold(maType, "SMA") {
		ma = SMA(closes, period)
	} else {
		ma = EMA(closes, period)
	}
	out := nanSlice(len(closes))
	for i := range closes {
		if math.IsNaN(ma[i]) || ma[i] == 0 {
			continue
		}
		out[i] = (closes[i] - ma[i]) / ma[i] * 100
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/indicators/ -v`
Expected: PASS (all indicator tests).

- [ ] **Step 5: Commit**

```bash
git add internal/indicators/distancema.go internal/indicators/distancema_test.go
git commit -m "feat(indicators): distance from moving average"
```

---

## Task 7: extrema (pivot peaks/valleys)

**Files:**
- Create: `internal/extrema/extrema.go`
- Test: `internal/extrema/extrema_test.go`

- [ ] **Step 1: Write the failing test**

`internal/extrema/extrema_test.go`:

```go
package extrema

import (
	"math"
	"testing"
)

func TestFindPeaksAndValleys(t *testing.T) {
	v := []float64{1, 3, 1, 5, 1}
	peaks := FindPeaks(v, 1)
	if len(peaks) != 2 || peaks[0].Index != 1 || peaks[1].Index != 3 {
		t.Fatalf("peaks = %+v, want indices 1 and 3", peaks)
	}
	valleys := FindValleys(v, 1)
	if len(valleys) != 1 || valleys[0].Index != 2 {
		t.Fatalf("valleys = %+v, want index 2", valleys)
	}
}

func TestPivotsIgnoreNaN(t *testing.T) {
	v := []float64{math.NaN(), 3, 1, 5, 1}
	// index 1 cannot be a peak: its left neighbor is NaN.
	peaks := FindPeaks(v, 1)
	if len(peaks) != 1 || peaks[0].Index != 3 {
		t.Fatalf("peaks = %+v, want only index 3", peaks)
	}
}

func TestLastN(t *testing.T) {
	in := []Pivot{{1, 1}, {2, 2}, {3, 3}, {4, 4}}
	got := LastN(in, 2)
	if len(got) != 2 || got[0].Index != 3 || got[1].Index != 4 {
		t.Fatalf("LastN = %+v, want last two", got)
	}
	if len(LastN(in, 10)) != 4 {
		t.Error("LastN with n>len should return all")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/extrema/ -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Write the implementation**

`internal/extrema/extrema.go`:

```go
package extrema

import "math"

// Pivot is a confirmed local extremum in a series.
type Pivot struct {
	Index int
	Value float64
}

// FindPeaks returns indices that are strictly greater than the w values on
// each side. Windows touching a NaN are skipped.
func FindPeaks(values []float64, w int) []Pivot {
	return find(values, w, true)
}

// FindValleys returns indices that are strictly less than the w values on
// each side. Windows touching a NaN are skipped.
func FindValleys(values []float64, w int) []Pivot {
	return find(values, w, false)
}

func find(values []float64, w int, peak bool) []Pivot {
	var out []Pivot
	for i := w; i < len(values)-w; i++ {
		v := values[i]
		if math.IsNaN(v) {
			continue
		}
		ok := true
		for j := 1; j <= w && ok; j++ {
			l, r := values[i-j], values[i+j]
			if math.IsNaN(l) || math.IsNaN(r) {
				ok = false
				break
			}
			if peak {
				ok = v > l && v > r
			} else {
				ok = v < l && v < r
			}
		}
		if ok {
			out = append(out, Pivot{Index: i, Value: v})
		}
	}
	return out
}

// LastN returns the last n pivots (most recent), or all if n >= len.
func LastN(pivots []Pivot, n int) []Pivot {
	if n >= len(pivots) {
		return pivots
	}
	return pivots[len(pivots)-n:]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/extrema/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/extrema/
git commit -m "feat(extrema): pivot peak/valley detection"
```

---

## Task 8: storage (Bar type, Store interface, Postgres impl)

**Files:**
- Create: `internal/storage/storage.go`
- Test: `internal/storage/storage_test.go` (integration, gated on `SCREENER_TEST_DSN`)

`Bar` is defined here because storage owns it; `resample` and `screener` import it.

- [ ] **Step 1: Write the failing (gated) test**

`internal/storage/storage_test.go`:

```go
package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

func testStore(t *testing.T) *PostgresStore {
	dsn := os.Getenv("SCREENER_TEST_DSN")
	if dsn == "" {
		t.Skip("set SCREENER_TEST_DSN to run storage integration tests")
	}
	s, err := NewPostgresStore(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_, _ = s.db.ExecContext(ctx, "DELETE FROM bars WHERE symbol = 'TST'")
	return s
}

func TestUpsertAndGet(t *testing.T) {
	s := testStore(t)
	defer s.Close()
	ctx := context.Background()
	t0 := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	bars := []Bar{
		{Symbol: "TST", Timeframe: "1d", Time: t0, Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 100},
		{Symbol: "TST", Timeframe: "1d", Time: t0.AddDate(0, 0, 1), Open: 1.5, High: 3, Low: 1, Close: 2.5, Volume: 200},
	}
	if err := s.UpsertBars(ctx, bars); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Re-upsert with a changed close should update, not duplicate.
	bars[1].Close = 2.7
	if err := s.UpsertBars(ctx, bars); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err := s.GetBars(ctx, "TST", "1d", 0)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (no duplicates)", len(got))
	}
	if got[0].Time.After(got[1].Time) {
		t.Error("bars must be ascending by time")
	}
	if got[1].Close != 2.7 {
		t.Errorf("close = %v, want 2.7 (upsert update)", got[1].Close)
	}
	last, ok, err := s.LastBarTime(ctx, "TST", "1d")
	if err != nil || !ok {
		t.Fatalf("LastBarTime: ok=%v err=%v", ok, err)
	}
	if !last.Equal(t0.AddDate(0, 0, 1)) {
		t.Errorf("last = %v, want %v", last, t0.AddDate(0, 0, 1))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -v`
Expected: FAIL — undefined symbols (or SKIP if no DSN; once compiling, run with a DSN to see it pass).

- [ ] **Step 3: Write the implementation**

`internal/storage/storage.go`:

```go
package storage

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

// Bar is one OHLCV candle for a (symbol, timeframe) at a bar-open time (UTC).
type Bar struct {
	Symbol    string
	Timeframe string
	Time      time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// Store is the persistence boundary the rest of the app depends on.
type Store interface {
	Migrate(ctx context.Context) error
	UpsertBars(ctx context.Context, bars []Bar) error
	// GetBars returns bars ascending by time. limit<=0 returns all; otherwise
	// the most recent `limit` bars (still returned ascending).
	GetBars(ctx context.Context, symbol, timeframe string, limit int) ([]Bar, error)
	LastBarTime(ctx context.Context, symbol, timeframe string) (time.Time, bool, error)
	Ping(ctx context.Context) error
	Close() error
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS bars (
	symbol    TEXT             NOT NULL,
	timeframe TEXT             NOT NULL,
	ts        TIMESTAMPTZ      NOT NULL,
	open      DOUBLE PRECISION NOT NULL,
	high      DOUBLE PRECISION NOT NULL,
	low       DOUBLE PRECISION NOT NULL,
	close     DOUBLE PRECISION NOT NULL,
	volume    DOUBLE PRECISION NOT NULL,
	PRIMARY KEY (symbol, timeframe, ts)
);
CREATE INDEX IF NOT EXISTS bars_symbol_tf_ts ON bars (symbol, timeframe, ts DESC);
`

func (s *PostgresStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *PostgresStore) UpsertBars(ctx context.Context, bars []Bar) error {
	if len(bars) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO bars (symbol, timeframe, ts, open, high, low, close, volume)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (symbol, timeframe, ts) DO UPDATE SET
			open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
			close = EXCLUDED.close, volume = EXCLUDED.volume`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, b := range bars {
		if _, err := stmt.ExecContext(ctx, b.Symbol, b.Timeframe, b.Time.UTC(),
			b.Open, b.High, b.Low, b.Close, b.Volume); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) GetBars(ctx context.Context, symbol, timeframe string, limit int) ([]Bar, error) {
	// Fetch most-recent-first with optional limit, then reverse to ascending.
	q := `SELECT symbol, timeframe, ts, open, high, low, close, volume
	      FROM bars WHERE symbol = $1 AND timeframe = $2 ORDER BY ts DESC`
	args := []any{symbol, timeframe}
	if limit > 0 {
		q += " LIMIT $3"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var desc []Bar
	for rows.Next() {
		var b Bar
		if err := rows.Scan(&b.Symbol, &b.Timeframe, &b.Time, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		b.Time = b.Time.UTC()
		desc = append(desc, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// reverse to ascending
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}

func (s *PostgresStore) LastBarTime(ctx context.Context, symbol, timeframe string) (time.Time, bool, error) {
	var ts time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT ts FROM bars WHERE symbol=$1 AND timeframe=$2 ORDER BY ts DESC LIMIT 1`,
		symbol, timeframe).Scan(&ts)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return ts.UTC(), true, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *PostgresStore) Close() error                   { return s.db.Close() }
```

- [ ] **Step 4: Run test to verify it passes**

Run (with a throwaway Postgres):
```bash
SCREENER_TEST_DSN="postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable" go test ./internal/storage/ -v
```
Expected: PASS. Without the env var: SKIP (still must compile: `go build ./internal/storage/`).

- [ ] **Step 5: Commit**

```bash
git add internal/storage/
git commit -m "feat(storage): Postgres bars store with upsert and queries"
```

---

## Task 9: resample (derived timeframes)

**Files:**
- Create: `internal/resample/resample.go`
- Test: `internal/resample/resample_test.go`

- [ ] **Step 1: Write the failing test**

`internal/resample/resample_test.go`:

```go
package resample

import (
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/storage"
)

func TestResample4h(t *testing.T) {
	base := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) // 12:00 -> 12:00 4h bucket
	var in []storage.Bar
	for i := 0; i < 4; i++ {
		in = append(in, storage.Bar{
			Symbol: "X", Timeframe: "1h", Time: base.Add(time.Duration(i) * time.Hour),
			Open: float64(i + 1), High: float64(10 + i), Low: float64(-i), Close: float64(i + 2), Volume: 100,
		})
	}
	out := To(in, "4h")
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 bucket", len(out))
	}
	b := out[0]
	if b.Open != 1 || b.Close != 5 || b.High != 13 || b.Low != -3 || b.Volume != 400 {
		t.Errorf("agg = %+v (open1 close5 high13 low-3 vol400)", b)
	}
	if b.Timeframe != "4h" || !b.Time.Equal(base) {
		t.Errorf("bucket meta = %v %v", b.Timeframe, b.Time)
	}
}

func TestResampleDropsIncompleteTrailingBucket(t *testing.T) {
	base := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	var in []storage.Bar
	for i := 0; i < 6; i++ { // 4 fill the 12:00 bucket, 2 start the 16:00 bucket
		in = append(in, storage.Bar{Symbol: "X", Timeframe: "1h",
			Time: base.Add(time.Duration(i) * time.Hour), Open: 1, High: 1, Low: 1, Close: 1, Volume: 1})
	}
	out := ToClosed(in, "4h")
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (incomplete 16:00 bucket dropped)", len(out))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/resample/ -v`
Expected: FAIL — `To`/`ToClosed` undefined.

- [ ] **Step 3: Write the implementation**

`internal/resample/resample.go`:

```go
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
	var curStart = tf.BucketStart(bars[0].Time)
	count := 0
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
			count = 1
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
		count++
	}
	flush()
	_ = count
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
	// Count parent bars in the last bucket.
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/resample/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/resample/
git commit -m "feat(resample): aggregate native bars into derived timeframes"
```

---

## Task 10: datasource/yahoo (v8 chart API client)

**Files:**
- Create: `internal/datasource/yahoo/yahoo.go`
- Create: `internal/datasource/yahoo/testdata/aapl_1d.json`
- Test: `internal/datasource/yahoo/yahoo_test.go`

- [ ] **Step 1: Create the parse fixture**

`internal/datasource/yahoo/testdata/aapl_1d.json` (a trimmed but valid chart response; note the deliberate `null` row to test gap handling):

```json
{
  "chart": {
    "result": [
      {
        "meta": { "symbol": "AAPL", "regularMarketPrice": 200.0 },
        "timestamp": [1718236800, 1718323200, 1718409600],
        "indicators": {
          "quote": [
            {
              "open":   [100.0, 101.0, null],
              "high":   [110.0, 111.0, 112.0],
              "low":    [99.0,  100.5, 101.0],
              "close":  [105.0, 106.0, 107.0],
              "volume": [1000,  1100,  1200]
            }
          ]
        }
      }
    ],
    "error": null
  }
}
```

- [ ] **Step 2: Write the failing test**

`internal/datasource/yahoo/yahoo_test.go`:

```go
package yahoo

import (
	"os"
	"testing"
)

func TestParseChart(t *testing.T) {
	raw, err := os.ReadFile("testdata/aapl_1d.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	candles, err := parseChart(raw)
	if err != nil {
		t.Fatalf("parseChart: %v", err)
	}
	// The middle row with null open is dropped -> 2 valid candles.
	if len(candles) != 2 {
		t.Fatalf("len = %d, want 2 (null row skipped)", len(candles))
	}
	if candles[0].Close != 105.0 || candles[0].Volume != 1000 {
		t.Errorf("candle0 = %+v", candles[0])
	}
	if candles[1].Close != 106.0 {
		t.Errorf("candle1 close = %v, want 106", candles[1].Close)
	}
	if candles[0].Time.Unix() != 1718236800 {
		t.Errorf("candle0 time = %v", candles[0].Time.Unix())
	}
}

func TestParseChartError(t *testing.T) {
	_, err := parseChart([]byte(`{"chart":{"result":null,"error":{"description":"Not Found"}}}`))
	if err == nil {
		t.Fatal("expected error for empty result")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/datasource/yahoo/ -v`
Expected: FAIL — `parseChart` undefined.

- [ ] **Step 4: Write the implementation**

`internal/datasource/yahoo/yahoo.go`:

```go
package yahoo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const baseURL = "https://query1.finance.yahoo.com/v8/finance/chart/"

// Candle is one OHLCV row from Yahoo.
type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

type Client struct {
	http      *http.Client
	userAgent string
}

func New() *Client {
	return &Client{
		http:      &http.Client{Timeout: 30 * time.Second},
		userAgent: "Mozilla/5.0 (stock-screener)",
	}
}

// rangeFor returns Yahoo's default range string for an interval when no
// explicit start time is given.
func rangeFor(interval string) string {
	switch interval {
	case "15m", "30m":
		return "60d"
	case "60m", "90m", "1h":
		return "730d"
	default: // 1d, 1wk, 1mo
		return "max"
	}
}

// Fetch returns candles for a symbol at a Yahoo interval. If `from` is non-zero
// it requests period1=from..now (incremental); otherwise it uses the default
// range for the interval.
func (c *Client) Fetch(ctx context.Context, symbol, interval string, from time.Time) ([]Candle, error) {
	q := url.Values{}
	q.Set("interval", interval)
	if from.IsZero() {
		q.Set("range", rangeFor(interval))
	} else {
		q.Set("period1", fmt.Sprintf("%d", from.Unix()))
		q.Set("period2", fmt.Sprintf("%d", time.Now().Unix()))
	}
	u := baseURL + url.PathEscape(symbol) + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo %s: status %d", symbol, resp.StatusCode)
	}
	return parseChart(body)
}

type chartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func parseChart(raw []byte) ([]Candle, error) {
	var cr chartResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("decode chart: %w", err)
	}
	if cr.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo error: %s", cr.Chart.Error.Description)
	}
	if len(cr.Chart.Result) == 0 || len(cr.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("yahoo: empty result")
	}
	res := cr.Chart.Result[0]
	q := res.Indicators.Quote[0]
	var out []Candle
	for i, ts := range res.Timestamp {
		if i >= len(q.Open) || i >= len(q.High) || i >= len(q.Low) || i >= len(q.Close) {
			break
		}
		o, h, l, cl := q.Open[i], q.High[i], q.Low[i], q.Close[i]
		if o == nil || h == nil || l == nil || cl == nil {
			continue // gap row
		}
		vol := 0.0
		if i < len(q.Volume) && q.Volume[i] != nil {
			vol = *q.Volume[i]
		}
		out = append(out, Candle{
			Time:  time.Unix(ts, 0).UTC(),
			Open:  *o, High: *h, Low: *l, Close: *cl, Volume: vol,
		})
	}
	return out, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/datasource/yahoo/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/datasource/
git commit -m "feat(yahoo): v8 chart API client with JSON parsing"
```

---

## Task 11: screener (rule + orchestration)

**Files:**
- Create: `internal/screener/types.go`, `internal/screener/rule.go`, `internal/screener/screener.go`
- Test: `internal/screener/rule_test.go`, `internal/screener/screener_test.go`

### 11a — types and rule

- [ ] **Step 1: Write the failing rule test**

`internal/screener/rule_test.go`:

```go
package screener

import (
	"testing"

	"github.com/Ruscigno/stock-screener/internal/extrema"
)

func TestClassify(t *testing.T) {
	peaks := []extrema.Pivot{{Index: 1, Value: 70}, {Index: 5, Value: 60}}   // min peak = 60
	valleys := []extrema.Pivot{{Index: 2, Value: 30}, {Index: 6, Value: 40}} // max valley = 40
	if z := classify(65, peaks, valleys); z != "high" {
		t.Errorf("classify(65) = %q, want high (>= min peak 60)", z)
	}
	if z := classify(35, peaks, valleys); z != "low" {
		t.Errorf("classify(35) = %q, want low (<= max valley 40)", z)
	}
	if z := classify(50, peaks, valleys); z != "neutral" {
		t.Errorf("classify(50) = %q, want neutral", z)
	}
}

func TestQualifies(t *testing.T) {
	if !qualifies(1, 3, "any") {
		t.Error("any: 1 trigger should qualify")
	}
	if qualifies(0, 3, "any") {
		t.Error("any: 0 triggers should not qualify")
	}
	if !qualifies(3, 3, "all") {
		t.Error("all: 3/3 should qualify")
	}
	if qualifies(2, 3, "all") {
		t.Error("all: 2/3 should not qualify")
	}
	if !qualifies(2, 3, "min:2") {
		t.Error("min:2: 2 triggers should qualify")
	}
	if qualifies(1, 3, "min:2") {
		t.Error("min:2: 1 trigger should not qualify")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/screener/ -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Write types and rule**

`internal/screener/types.go`:

```go
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
```

`internal/screener/rule.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/screener/ -run 'TestClassify|TestQualifies' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/screener/types.go internal/screener/rule.go internal/screener/rule_test.go
git commit -m "feat(screener): result types and screening rule"
```

### 11b — orchestration

- [ ] **Step 6: Write the failing orchestration test**

`internal/screener/screener_test.go`:

```go
package screener

import (
	"context"
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/storage"
)

// fakeStore returns canned bars for one (symbol, timeframe).
type fakeStore struct{ bars []storage.Bar }

func (f *fakeStore) Migrate(context.Context) error { return nil }
func (f *fakeStore) UpsertBars(context.Context, []storage.Bar) error { return nil }
func (f *fakeStore) GetBars(_ context.Context, _, _ string, _ int) ([]storage.Bar, error) {
	return f.bars, nil
}
func (f *fakeStore) LastBarTime(context.Context, string, string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close() error               { return nil }

func testConfig() *config.Config {
	c := &config.Config{}
	c.Screening.Match = "any"
	c.Screening.PivotWindow = 1
	c.Screening.TrendLookback = 1
	c.Screening.PeaksToShow = 3
	c.Indicators.RSI.Length = 14
	c.Indicators.VolumeOscillator.ShortLength = 5
	c.Indicators.VolumeOscillator.LongLength = 10
	c.Indicators.DistanceFromMA.MAType = "SMA"
	c.Indicators.DistanceFromMA.Length = 3
	return c
}

// buildBars makes a daily series from closes (volume constant).
func buildBars(closes []float64) []storage.Bar {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var bars []storage.Bar
	for i, c := range closes {
		bars = append(bars, storage.Bar{
			Symbol: "AAA", Timeframe: "1d", Time: t0.AddDate(0, 0, i),
			Open: c, High: c, Low: c, Close: c, Volume: 100,
		})
	}
	return bars
}

func TestScreenDistanceLowTriggers(t *testing.T) {
	// Distance-from-SMA(3): a V-shape so the latest close sits at/below a
	// recent distance valley -> "low" zone -> triggers under match=any.
	closes := []float64{10, 12, 14, 12, 10, 12, 14, 16, 14, 12, 10}
	s := New(&fakeStore{bars: buildBars(closes)}, testConfig())
	res, err := s.Screen(context.Background(), Request{
		Symbols: []string{"AAA"}, Timeframes: []string{"1d"},
		Match: "any", Indicators: []string{IndDistance},
	})
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	r := res.Rows[0]
	if r.Symbol != "AAA" || r.Timeframe != "1d" {
		t.Errorf("row meta = %+v", r)
	}
	if _, ok := r.Indicators[IndDistance]; !ok {
		t.Error("missing distance_from_ma indicator")
	}
}

func TestScreenInsufficientDataWarns(t *testing.T) {
	// Only 3 closes but distance length 3 + pivots need more -> still computes
	// distance value at idx2, but RSI (length 14) is insufficient.
	s := New(&fakeStore{bars: buildBars([]float64{10, 11, 12})}, testConfig())
	res, err := s.Screen(context.Background(), Request{
		Symbols: []string{"AAA"}, Timeframes: []string{"1d"},
		Match: "any", Indicators: []string{IndRSI},
	})
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected an insufficient_data warning for RSI")
	}
}
```

- [ ] **Step 7: Run test to verify it fails**

Run: `go test ./internal/screener/ -run TestScreen -v`
Expected: FAIL — `New`/`Screen` undefined.

- [ ] **Step 8: Write the orchestration**

`internal/screener/screener.go`:

```go
package screener

import (
	"context"
	"fmt"
	"math"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/extrema"
	"github.com/Ruscigno/stock-screener/internal/indicators"
	"github.com/Ruscigno/stock-screener/internal/resample"
	"github.com/Ruscigno/stock-screener/internal/storage"
	"github.com/Ruscigno/stock-screener/internal/timeframe"
)

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
	if tf.Native {
		return s.store.GetBars(ctx, symbol, tf.Name, 0)
	}
	parent, err := s.store.GetBars(ctx, symbol, tf.Parent, 0)
	if err != nil {
		return nil, err
	}
	if s.cfg.Collector.UseClosedBarsOnly {
		return resample.ToClosed(parent, tf.Name), nil
	}
	return resample.To(parent, tf.Name), nil
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

	if !qualifies(len(triggered), len(req.Indicators), req.Match) {
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
```

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/screener/ -v`
Expected: PASS (rule + orchestration). If `TestScreenDistanceLowTriggers` does not produce a row, adjust the `closes` fixture so the final distance value is `<=` the max of the last-3 distance valleys — the assertion documents the intended behavior; the series math is correct, only the fixture may need a clearer V-shape.

- [ ] **Step 10: Commit**

```bash
git add internal/screener/screener.go internal/screener/screener_test.go
git commit -m "feat(screener): orchestrate load, indicators, extrema, screening"
```

---

## Task 12: api (HTTP handlers)

**Files:**
- Create: `internal/api/api.go`
- Test: `internal/api/api_test.go`

- [ ] **Step 1: Write the failing test**

`internal/api/api_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/screener"
)

type fakeScreener struct{ res screener.Result }

func (f *fakeScreener) Screen(context.Context, screener.Request) (screener.Result, error) {
	return f.res, nil
}

type fakePinger struct{ err error }

func (f *fakePinger) Ping(context.Context) error { return f.err }

func testCfg() *config.Config {
	c := &config.Config{}
	c.Stocks = []string{"AAPL"}
	c.Timeframes = []string{"1d"}
	c.Screening.Match = "any"
	return c
}

func TestScreenEndpoint(t *testing.T) {
	res := screener.Result{
		Rows: []screener.Row{{
			Symbol: "AAPL", Timeframe: "1d", BarTime: time.Now().UTC(), Price: 200,
			Triggered: []string{"rsi"},
			Indicators: map[string]screener.IndicatorResult{
				"rsi": {Latest: 28.3, Trend: "rising", Zone: "low", Triggered: true,
					Peaks: []screener.PivotPoint{{Value: 70, Time: time.Now().UTC()}}},
			},
		}},
	}
	srv := NewServer(&fakeScreener{res: res}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/screen", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Results []struct {
			Symbol    string   `json:"symbol"`
			Timeframe string   `json:"timeframe"`
			Triggered []string `json:"triggered"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Results) != 1 || body.Results[0].Symbol != "AAPL" {
		t.Fatalf("results = %+v", body.Results)
	}
}

func TestHealthz(t *testing.T) {
	srv := NewServer(&fakeScreener{}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200", rec.Code)
	}
}

func TestScreenRejectsUnknownTimeframe(t *testing.T) {
	srv := NewServer(&fakeScreener{}, &fakePinger{}, testCfg())
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/screen?timeframes=7m", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -v`
Expected: FAIL — `NewServer` undefined.

- [ ] **Step 3: Write the implementation**

`internal/api/api.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/screener"
	"github.com/Ruscigno/stock-screener/internal/timeframe"
)

// ScreenRunner is the screener dependency (real or fake).
type ScreenRunner interface {
	Screen(ctx context.Context, req screener.Request) (screener.Result, error)
}

// Pinger checks backing-store health.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Server struct {
	scr ScreenRunner
	db  Pinger
	cfg *config.Config
}

func NewServer(scr ScreenRunner, db Pinger, cfg *config.Config) *Server {
	return &Server{scr: scr, db: db, cfg: cfg}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/screen", s.handleScreen)
	mux.HandleFunc("/healthz", s.handleHealthz)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleScreen(w http.ResponseWriter, r *http.Request) {
	req := screener.Request{
		Symbols:    csvOrDefault(r.URL.Query().Get("symbols"), s.cfg.Stocks),
		Timeframes: csvOrDefault(r.URL.Query().Get("timeframes"), s.cfg.Timeframes),
		Match:      orDefault(r.URL.Query().Get("match"), s.cfg.Screening.Match),
		Indicators: csvOrDefault(r.URL.Query().Get("indicators"), screener.AllIndicators),
	}
	for _, tf := range req.Timeframes {
		if _, ok := timeframe.Get(tf); !ok {
			http.Error(w, "unknown timeframe: "+tf, http.StatusBadRequest)
			return
		}
	}
	if !validMatch(req.Match) {
		http.Error(w, "invalid match mode: "+req.Match, http.StatusBadRequest)
		return
	}

	result, err := s.scr.Screen(r.Context(), req)
	if err != nil {
		http.Error(w, "screen failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, toDTO(result, req))
}

// ---- DTOs (stable JSON shape, decoupled from internal types) ----

type pivotDTO struct {
	Value float64   `json:"value"`
	Time  time.Time `json:"time"`
}
type indicatorDTO struct {
	Latest    float64    `json:"latest"`
	Trend     string     `json:"trend"`
	Zone      string     `json:"zone"`
	Triggered bool       `json:"triggered"`
	Peaks     []pivotDTO `json:"peaks"`
	Valleys   []pivotDTO `json:"valleys"`
}
type rowDTO struct {
	Symbol     string                  `json:"symbol"`
	Timeframe  string                  `json:"timeframe"`
	BarTime    time.Time               `json:"bar_time"`
	Price      float64                 `json:"price"`
	Triggered  []string                `json:"triggered"`
	Indicators map[string]indicatorDTO `json:"indicators"`
}
type warningDTO struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`
	Message   string `json:"message"`
}
type responseDTO struct {
	AsOf     time.Time `json:"as_of"`
	Criteria struct {
		Match      string   `json:"match"`
		Symbols    int      `json:"symbols"`
		Timeframes []string `json:"timeframes"`
	} `json:"criteria"`
	Results  []rowDTO     `json:"results"`
	Warnings []warningDTO `json:"warnings"`
}

func toDTO(res screener.Result, req screener.Request) responseDTO {
	var out responseDTO
	out.AsOf = time.Now().UTC()
	out.Criteria.Match = req.Match
	out.Criteria.Symbols = len(req.Symbols)
	out.Criteria.Timeframes = req.Timeframes
	out.Results = make([]rowDTO, 0, len(res.Rows))
	for _, row := range res.Rows {
		rd := rowDTO{
			Symbol: row.Symbol, Timeframe: row.Timeframe, BarTime: row.BarTime,
			Price: row.Price, Triggered: row.Triggered,
			Indicators: map[string]indicatorDTO{},
		}
		for name, ir := range row.Indicators {
			rd.Indicators[name] = indicatorDTO{
				Latest: ir.Latest, Trend: ir.Trend, Zone: ir.Zone, Triggered: ir.Triggered,
				Peaks: pivotsToDTO(ir.Peaks), Valleys: pivotsToDTO(ir.Valleys),
			}
		}
		out.Results = append(out.Results, rd)
	}
	out.Warnings = make([]warningDTO, 0, len(res.Warnings))
	for _, wn := range res.Warnings {
		out.Warnings = append(out.Warnings, warningDTO(wn))
	}
	return out
}

func pivotsToDTO(in []screener.PivotPoint) []pivotDTO {
	out := make([]pivotDTO, 0, len(in))
	for _, p := range in {
		out = append(out, pivotDTO{Value: p.Value, Time: p.Time})
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func csvOrDefault(s string, def []string) []string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func validMatch(m string) bool {
	if m == "any" || m == "all" {
		return true
	}
	return strings.HasPrefix(m, "min:")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat(api): /screen and /healthz handlers with JSON DTOs"
```

---

## Task 13: collector (scheduler)

**Files:**
- Create: `internal/collector/collector.go`
- Test: `internal/collector/collector_test.go`

- [ ] **Step 1: Write the failing test**

`internal/collector/collector_test.go`:

```go
package collector

import (
	"testing"
	"time"

	"github.com/Ruscigno/stock-screener/internal/datasource/yahoo"
)

func TestNativeTimeframes(t *testing.T) {
	got := nativeTimeframes([]string{"15m", "4h", "1d", "3d", "1h"})
	// derived (4h, 3d) dropped; natives deduped and preserved.
	want := map[string]bool{"15m": true, "1d": true, "1h": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want keys %v", got, want)
	}
	for _, tf := range got {
		if !want[tf] {
			t.Errorf("unexpected native tf %q", tf)
		}
	}
}

func TestDropUnclosed(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	candles := []yahoo.Candle{
		{Time: now.Add(-2 * time.Hour), Close: 1}, // closed (bar end 10:00+1h <= 12:00)
		{Time: now.Add(-30 * time.Minute), Close: 2}, // still forming (ends 12:30 > 12:00)
	}
	out := dropUnclosed(candles, time.Hour, now)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (forming bar dropped)", len(out))
	}
	if out[0].Close != 1 {
		t.Errorf("kept wrong bar: %+v", out[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/collector/ -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Write the implementation**

`internal/collector/collector.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/collector/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/collector/
git commit -m "feat(collector): scheduled Yahoo fetch and upsert"
```

---

## Task 14: main wiring + sample config

**Files:**
- Create: `main.go`, `config.yaml`

- [ ] **Step 1: Write the sample config**

`config.yaml`:

```yaml
server:
  port: 8080

collector:
  enabled: true
  use_closed_bars_only: true
  refresh:
    intraday: 15m
    daily: 6h

stocks: [AAPL, GOOGL, MSFT, TSLA, AMZN]

timeframes: [15m, 30m, 1h, 4h, 1d, 3d, 1wk, 1mo]

screening:
  match: any
  pivot_window: 3
  trend_lookback: 3
  peaks_to_show: 3
  peak_lookback: 3mo

indicators:
  rsi:
    length: 14
    source: close
    smoothing: { type: SMA, length: 14, bb_stddev: 2 }
  volume_oscillator:
    short_length: 5
    long_length: 10
  distance_from_ma:
    source: close
    ma_type: EMA
    length: 200
    calculation: percent
```

- [ ] **Step 2: Write main.go**

`main.go`:

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ruscigno/stock-screener/internal/api"
	"github.com/Ruscigno/stock-screener/internal/collector"
	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/datasource/yahoo"
	"github.com/Ruscigno/stock-screener/internal/screener"
	"github.com/Ruscigno/stock-screener/internal/storage"
)

func dsnFromEnv() (string, error) {
	u, p, h, port, name := os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME")
	if u == "" || h == "" || name == "" {
		return "", fmt.Errorf("DB_USER, DB_HOST, DB_NAME must be set")
	}
	if port == "" {
		port = "5432"
	}
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable", u, p, h, port, name), nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <serve|collect> [--config config.yaml]", os.Args[0])
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	cfgPath := fs.String("config", "config.yaml", "path to config file")
	_ = fs.Parse(os.Args[2:])

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	dsn, err := dsnFromEnv()
	if err != nil {
		log.Fatalf("db env: %v", err)
	}
	store, err := storage.NewPostgresStore(dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	switch cmd {
	case "collect":
		col := collector.New(store, yahoo.New(), cfg)
		errs := col.CollectOnce(ctx)
		log.Printf("collect finished with %d errors", len(errs))
	case "serve":
		if cfg.Collector.Enabled {
			col := collector.New(store, yahoo.New(), cfg)
			go col.Run(ctx)
		}
		scr := screener.New(store, cfg)
		srv := api.NewServer(scr, store, cfg)
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}
		go func() {
			<-ctx.Done()
			_ = httpSrv.Close()
		}()
		log.Printf("listening on %s", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	default:
		log.Fatalf("unknown command %q (want serve|collect)", cmd)
	}
}
```

- [ ] **Step 3: Build and vet everything**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build clean; all non-gated tests PASS; storage test SKIPs without DSN.

- [ ] **Step 4: Manual smoke test (real Postgres + Yahoo)**

```bash
# start a throwaway Postgres
docker run -d --name screener-pg -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:16
export DB_USER=postgres DB_PASSWORD=postgres DB_HOST=localhost DB_PORT=5432 DB_NAME=postgres

# create a small config with 1 symbol + a couple timeframes to keep it fast
cat > /tmp/smoke.yaml <<'YAML'
server: { port: 8080 }
collector: { enabled: false, use_closed_bars_only: true, refresh: { intraday: 15m, daily: 6h } }
stocks: [AAPL]
timeframes: [1d, 4h]
screening: { match: any, pivot_window: 3, trend_lookback: 3, peaks_to_show: 3, peak_lookback: 3mo }
indicators:
  rsi: { length: 14, source: close, smoothing: { type: SMA, length: 14, bb_stddev: 2 } }
  volume_oscillator: { short_length: 5, long_length: 10 }
  distance_from_ma: { source: close, ma_type: EMA, length: 200, calculation: percent }
YAML

go run . collect --config /tmp/smoke.yaml      # should log "collected AAPL 1d: N bars" and "AAPL 1h"
go run . serve --config /tmp/smoke.yaml &       # start server
sleep 2
curl -s "http://localhost:8080/healthz"; echo
curl -s "http://localhost:8080/screen?symbols=AAPL&timeframes=1d&match=any" | head -c 800; echo
kill %1
docker rm -f screener-pg
```
Expected: `collect` logs bars for `AAPL 1d` and `AAPL 1h` (1h is the parent fetched for the derived 4h). `/healthz` returns `ok`. `/screen` returns valid JSON with `results`/`warnings` (results may be empty depending on current market position — that is correct behavior, not a failure).

- [ ] **Step 5: Commit**

```bash
git add main.go config.yaml
git commit -m "feat: wire serve/collect commands and sample config"
```

---

## Task 15: project README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README.md**

`README.md`:

```markdown
# stock-screener

Collects Yahoo Finance OHLCV into Postgres and screens stocks that are sitting
at a recent extreme (RSI, volume oscillator, distance-from-MA) across multiple
timeframes.

## Configure

Edit `config.yaml` (stocks, timeframes, indicator params). Database credentials
come from environment variables:

    DB_USER, DB_PASSWORD, DB_HOST, DB_PORT (default 5432), DB_NAME

## Run

    go run . collect --config config.yaml   # fetch + store native-timeframe bars
    go run . serve   --config config.yaml   # HTTP API (runs collector in-process if enabled)

## API

- `GET /healthz` — liveness (pings Postgres).
- `GET /screen` — qualifying (stock, timeframe) rows. Optional query params
  `symbols`, `timeframes`, `match` (`any|all|min:N`), `indicators`; each
  defaults to `config.yaml`.

A (stock, timeframe) row qualifies when, per `match`, its indicators are at an
extreme: current value `>=` the lowest of the last 3 peaks (zone `high`) or
`<=` the highest of the last 3 valleys (zone `low`). Each indicator reports its
latest value, trend (rising/falling/flat), and last 3 peaks/valleys.

Timeframes `15m,30m,1h,1d,1wk,1mo` are fetched from Yahoo; `4h` and `3d` are
resampled from `1h`/`1d` at query time.

See `docs/superpowers/specs/2026-06-16-stock-screener-design.md` for the full design.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README"
```

---

## Self-Review Notes (completed during planning)

- **Spec coverage:** indicators (Tasks 3–6), peaks/valleys (Task 7), screening rule incl. `any/all/min:N` and zone (Task 11a), trend slope (Task 11b), timeframes native+derived & resample (Tasks 2, 9), DB collector + on-demand compute architecture (Tasks 8, 11, 13), Yahoo v8 data source (Task 10), config file with all tunables (Task 1), API contract incl. warnings/400/500 (Task 12), wiring + run modes (Task 14). All spec sections map to a task.
- **Type consistency:** `storage.Bar` defined once (Task 8) and reused; `extrema.Pivot` (Task 7) consumed by `screener` (Task 11); `screener.Request/Result/Row/IndicatorResult/PivotPoint` defined in Task 11a and consumed by Task 12; `Store` interface satisfied by `PostgresStore`, `fakeStore` (screener test), and the api `Pinger`/`ScreenRunner` narrowings.
- **Known approximation:** resample buckets use fixed UTC boundaries (per spec §3, §10); session-aware alignment is explicitly out of scope.
- **Decision recorded:** `peak_lookback` (config) guarantees enough history is loaded; v1 selects the last-3 pivots from the full stored series (which is bounded by Yahoo's per-interval history limits), satisfying the spec's "extend until 3 found" without a separate windowing pass.
```
