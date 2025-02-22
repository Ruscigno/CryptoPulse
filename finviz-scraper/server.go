package finvizserver

import (
	"os"
	"os/signal"

	"github.com/Ruscigno/cryptopulse/api"
	"github.com/Ruscigno/cryptopulse/finviz-scraper/config"
	"github.com/Ruscigno/cryptopulse/finviz-scraper/crawler"
	"github.com/Ruscigno/cryptopulse/finviz-scraper/storage"
	"github.com/Ruscigno/cryptopulse/finviz-scraper/worker"
	"go.uber.org/zap"
)

func StartFinvizScraperServer() {
	cfg := config.Load()

	// Initialize MongoDB
	store, err := storage.NewMongoStorage(cfg.MongoURI, cfg.Database)
	if err != nil {
		zap.L().Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer store.Close()

	// Initialize crawler and worker pool
	cr := crawler.NewCrawler(store)
	wq := worker.NewWorkQueue(1, cr) // 10 workers?
	defer wq.Stop()

	// Start API server
	router := api.SetupRouter(store, wq)
	go func() {
		if err := router.Run(":3001"); err != nil {
			zap.L().Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan
	zap.L().Info("Shutting down...")
}
