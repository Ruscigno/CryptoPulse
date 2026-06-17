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
