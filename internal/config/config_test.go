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
		"1y":  365 * 24 * time.Hour,
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

func validBaseConfig() Config {
	var c Config
	c.Stocks = []string{"AAPL"}
	c.Timeframes = []string{"1d"}
	c.Screening.Match = "any"
	c.Screening.PivotWindow = 3
	c.Screening.TrendLookback = 3
	c.Screening.PeaksToShow = 3
	c.Indicators.RSI.Length = 14
	c.Indicators.VolumeOscillator.ShortLength = 5
	c.Indicators.VolumeOscillator.LongLength = 10
	c.Indicators.DistanceFromMA.Length = 200
	return c
}

func TestValidateRules(t *testing.T) {
	base := validBaseConfig()
	if err := base.validate(); err != nil {
		t.Fatalf("valid base config rejected: %v", err)
	}
	mutate := map[string]func(*Config){
		"empty stocks":       func(c *Config) { c.Stocks = nil },
		"empty timeframes":   func(c *Config) { c.Timeframes = nil },
		"unknown timeframe":  func(c *Config) { c.Timeframes = []string{"1day"} },
		"bad match":          func(c *Config) { c.Screening.Match = "nope" },
		"min:0 match":        func(c *Config) { c.Screening.Match = "min:0" },
		"pivot_window 0":     func(c *Config) { c.Screening.PivotWindow = 0 },
		"trend_lookback 0":   func(c *Config) { c.Screening.TrendLookback = 0 },
		"peaks_to_show 0":    func(c *Config) { c.Screening.PeaksToShow = 0 },
		"rsi length 1":       func(c *Config) { c.Indicators.RSI.Length = 1 },
		"volosc short>=long": func(c *Config) { c.Indicators.VolumeOscillator.ShortLength = 10 },
		"distance length 1":  func(c *Config) { c.Indicators.DistanceFromMA.Length = 1 },
	}
	for name, m := range mutate {
		c := validBaseConfig()
		m(&c)
		if err := c.validate(); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}
