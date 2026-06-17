package main

import (
	"strings"
	"testing"
)

func TestDSNFromEnv(t *testing.T) {
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PASSWORD", "p")
	t.Setenv("DB_HOST", "h")
	t.Setenv("DB_NAME", "n")
	t.Setenv("DB_PORT", "")    // default port
	t.Setenv("DB_SSLMODE", "") // default sslmode

	dsn, err := dsnFromEnv()
	if err != nil {
		t.Fatalf("dsnFromEnv: %v", err)
	}
	if dsn != "postgresql://u:p@h:5432/n?sslmode=require" {
		t.Errorf("dsn = %q", dsn)
	}
}

func TestDSNFromEnvSSLModeOverride(t *testing.T) {
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PASSWORD", "p")
	t.Setenv("DB_HOST", "h")
	t.Setenv("DB_NAME", "n")
	t.Setenv("DB_PORT", "6543")
	t.Setenv("DB_SSLMODE", "disable")

	dsn, err := dsnFromEnv()
	if err != nil {
		t.Fatalf("dsnFromEnv: %v", err)
	}
	if dsn != "postgresql://u:p@h:6543/n?sslmode=disable" {
		t.Errorf("dsn = %q", dsn)
	}
}

func TestDSNFromEnvMissingRequired(t *testing.T) {
	t.Setenv("DB_USER", "")
	t.Setenv("DB_HOST", "h")
	t.Setenv("DB_NAME", "n")
	if _, err := dsnFromEnv(); err == nil || !strings.Contains(err.Error(), "must be set") {
		t.Errorf("expected missing-env error, got %v", err)
	}
}
