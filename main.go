package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ruscigno/stock-screener/internal/api"
	"github.com/Ruscigno/stock-screener/internal/collector"
	"github.com/Ruscigno/stock-screener/internal/config"
	"github.com/Ruscigno/stock-screener/internal/datasource/yahoo"
	"github.com/Ruscigno/stock-screener/internal/screener"
	"github.com/Ruscigno/stock-screener/internal/storage"
)

// dsnFromEnv builds the Postgres DSN from environment variables. TLS mode is
// configurable via DB_SSLMODE and defaults to "require" (secure); local setups
// without TLS must opt out explicitly with DB_SSLMODE=disable.
func dsnFromEnv() (string, error) {
	u, p, h, port, name := os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME")
	if u == "" || h == "" || name == "" {
		return "", fmt.Errorf("DB_USER, DB_HOST, DB_NAME must be set")
	}
	if port == "" {
		port = "5432"
	}
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "require"
	}
	// Build via url.URL so credentials containing @ : / ? # % are
	// percent-encoded rather than corrupting the DSN that lib/pq parses.
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(u, p),
		Host:     net.JoinHostPort(h, port),
		Path:     "/" + name,
		RawQuery: url.Values{"sslmode": {sslmode}}.Encode(),
	}
	return dsn.String(), nil
}

func main() { os.Exit(run(os.Args)) }

// run wires everything and returns a process exit code (0 ok, 1 runtime error,
// 2 usage error). Using a returned code keeps deferred cleanup (store.Close)
// running, unlike os.Exit inside main.
func run(args []string) int {
	if len(args) < 2 {
		log.Printf("usage: %s <serve|collect> [--config config.yaml]", args[0])
		return 2
	}
	cmd := args[1]
	if cmd != "serve" && cmd != "collect" {
		log.Printf("unknown command %q (want serve|collect)", cmd)
		return 2
	}
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	cfgPath := fs.String("config", "config.yaml", "path to config file")
	_ = fs.Parse(args[2:])

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Printf("config: %v", err)
		return 1
	}
	dsn, err := dsnFromEnv()
	if err != nil {
		log.Printf("db env: %v", err)
		return 1
	}
	store, err := storage.NewPostgresStore(dsn)
	if err != nil {
		log.Printf("db connect: %v", err)
		return 1
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := store.Migrate(ctx); err != nil {
		log.Printf("migrate: %v", err)
		return 1
	}

	switch cmd {
	case "collect":
		errs := collector.New(store, yahoo.New(), cfg).CollectOnce(ctx)
		for _, e := range errs {
			log.Printf("collect error: %v", e)
		}
		if len(errs) > 0 {
			log.Printf("collect finished with %d error(s)", len(errs))
			return 1
		}
		log.Printf("collect finished: ok")
		return 0
	case "serve":
		return serve(ctx, cfg, store)
	}
	return 0
}

func serve(ctx context.Context, cfg *config.Config, store *storage.PostgresStore) int {
	var worker func(context.Context)
	if cfg.Collector.Enabled {
		worker = collector.New(store, yahoo.New(), cfg).Run
	}
	scr := screener.New(store, cfg)
	srv := api.NewServer(scr, store, cfg)
	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("listening on %s", httpSrv.Addr)
	return serveLoop(ctx, httpSrv, worker)
}

// httpServer is the slice of *http.Server that serveLoop needs (so tests can
// inject a fake).
type httpServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// serveLoop runs srv until ctx is cancelled (then gracefully shuts it down) or
// it fails. The optional background worker runs on a child context and is
// always cancelled and drained before serveLoop returns — so the caller's
// deferred store.Close() never races an in-flight collector cycle, on the
// normal-shutdown path AND the server-error path. Returns a process exit code.
func serveLoop(ctx context.Context, srv httpServer, worker func(context.Context)) int {
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()

	var workerDone chan struct{}
	if worker != nil {
		workerDone = make(chan struct{})
		go func() {
			worker(workerCtx)
			close(workerDone)
		}()
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	code := 0
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server: %v", err)
		code = 1
	}
	cancelWorker() // stop the worker even if the server failed before ctx cancel
	if workerDone != nil {
		<-workerDone // drain before returning
	}
	return code
}
