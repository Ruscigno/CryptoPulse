package main

import (
	"net/http"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
	"github.com/Ruscigno/CryptoPulse/pkg/endpoint"
	"github.com/Ruscigno/CryptoPulse/pkg/service"
	httptransport "github.com/Ruscigno/CryptoPulse/pkg/transport/http"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize service
	svc := service.NewService()

	// Create endpoints
	endpoints := endpoint.MakeEndpoints(svc)

	// Set up HTTP handler
	handler := httptransport.NewHTTPHandler(endpoints)

	// Start server
	logger.Info("Starting server", zap.String("port", cfg.HTTPPort))
	if err := http.ListenAndServe(cfg.HTTPPort, handler); err != nil {
		logger.Fatal("Server failed", zap.Error(err))
	}
}
