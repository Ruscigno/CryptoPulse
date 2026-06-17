package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
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

// fakeHTTPServer lets serveLoop be tested without binding a real port.
type fakeHTTPServer struct {
	started  chan struct{}
	shutdown chan struct{}
	failErr  error // if set, ListenAndServe returns it immediately
}

func newFakeHTTPServer() *fakeHTTPServer {
	return &fakeHTTPServer{started: make(chan struct{}), shutdown: make(chan struct{})}
}

func (f *fakeHTTPServer) ListenAndServe() error {
	close(f.started)
	if f.failErr != nil {
		return f.failErr
	}
	<-f.shutdown
	return http.ErrServerClosed
}

func (f *fakeHTTPServer) Shutdown(context.Context) error {
	close(f.shutdown)
	return nil
}

func TestServeLoopDrainsWorkerOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	srv := newFakeHTTPServer()
	ran := make(chan struct{})
	returned := make(chan struct{})
	worker := func(c context.Context) {
		close(ran)
		<-c.Done() // worker stops when its context is cancelled
		close(returned)
	}

	done := make(chan int, 1)
	go func() { done <- serveLoop(ctx, srv, worker) }()

	<-srv.started
	<-ran
	cancel() // trigger graceful shutdown

	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveLoop did not return after ctx cancel")
	}
	select {
	case <-returned:
	default:
		t.Error("worker was not drained before serveLoop returned")
	}
}

func TestServeLoopReturns1AndDrainsOnServerError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := newFakeHTTPServer()
	srv.failErr = errServe
	returned := make(chan struct{})
	worker := func(c context.Context) {
		<-c.Done() // must be cancelled by serveLoop even though ctx wasn't
		close(returned)
	}

	done := make(chan int, 1)
	go func() { done <- serveLoop(ctx, srv, worker) }()

	select {
	case code := <-done:
		if code != 1 {
			t.Errorf("code = %d, want 1 on server error", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveLoop did not return on server error")
	}
	select {
	case <-returned:
	default:
		t.Error("worker not drained on server-error path (store.Close race)")
	}
}

var errServe = serveErr("listen: address already in use")

type serveErr string

func (e serveErr) Error() string { return string(e) }
