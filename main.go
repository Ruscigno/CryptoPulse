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
	// Run the collector in-process, but track its goroutine so we can wait for
	// it to drain before the caller's deferred store.Close() runs (otherwise an
	// in-flight Fetch/UpsertBars races db.Close on shutdown).
	var collectorDone chan struct{}
	if cfg.Collector.Enabled {
		col := collector.New(store, yahoo.New(), cfg)
		collectorDone = make(chan struct{})
		go func() {
			col.Run(ctx)
			close(collectorDone)
		}()
	}

	scr := screener.New(store, cfg)
	srv := api.NewServer(scr, store, cfg)
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	log.Printf("listening on %s", addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server: %v", err)
		return 1
	}
	if collectorDone != nil {
		<-collectorDone // drain the collector before store.Close()
	}
	return 0
}
