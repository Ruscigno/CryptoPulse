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
