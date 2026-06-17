package main

import (
	"net/url"
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
	if dsn != "postgres://u:p@h:5432/n?sslmode=require" {
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
	if dsn != "postgres://u:p@h:6543/n?sslmode=disable" {
		t.Errorf("dsn = %q", dsn)
	}
}

// TestDSNFromEnvSpecialCharPassword guards against DSN corruption when the
// password contains characters that are significant in a URL (@ : / ? #).
func TestDSNFromEnvSpecialCharPassword(t *testing.T) {
	const pw = "p@ss:w/rd?x#y"
	t.Setenv("DB_USER", "user")
	t.Setenv("DB_PASSWORD", pw)
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_NAME", "screener")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_SSLMODE", "require")

	dsn, err := dsnFromEnv()
	if err != nil {
		t.Fatalf("dsnFromEnv: %v", err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("DSN does not parse: %v (dsn=%q)", err, dsn)
	}
	if parsed.Host != "db.example.com:5432" {
		t.Errorf("host = %q, want db.example.com:5432 (password leaked into host?)", parsed.Host)
	}
	gotPw, _ := parsed.User.Password()
	if gotPw != pw {
		t.Errorf("password round-trip = %q, want %q", gotPw, pw)
	}
	if parsed.User.Username() != "user" {
		t.Errorf("user = %q, want user", parsed.User.Username())
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

func TestRunUsageErrors(t *testing.T) {
	if code := run([]string{"prog"}); code != 2 {
		t.Errorf("no subcommand: code = %d, want 2", code)
	}
	if code := run([]string{"prog", "bogus"}); code != 2 {
		t.Errorf("bad subcommand: code = %d, want 2", code)
	}
}
