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
