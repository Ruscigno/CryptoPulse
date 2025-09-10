package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
	"github.com/Ruscigno/CryptoPulse/pkg/database"
	"github.com/Ruscigno/CryptoPulse/pkg/endpoint"
	"github.com/Ruscigno/CryptoPulse/pkg/query"
	"github.com/Ruscigno/CryptoPulse/pkg/repository"
	"github.com/Ruscigno/CryptoPulse/pkg/service"
	httptransport "github.com/Ruscigno/CryptoPulse/pkg/transport/http"
	"github.com/Ruscigno/CryptoPulse/pkg/tx"
	"github.com/Ruscigno/CryptoPulse/pkg/wallet"
	"go.uber.org/zap"
)

// Build-time variables (set via ldflags)
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	// Parse command line flags
	var (
		healthCheck = flag.Bool("health-check", false, "Perform health check and exit")
		showVersion = flag.Bool("version", false, "Show version information and exit")
	)
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("dYdX Order Routing Service\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		os.Exit(0)
	}

	// Handle health check flag
	if *healthCheck {
		performHealthCheck()
		return
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database
	db, err := database.NewDB(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// Initialize repository
	orderRepo := repository.NewOrderRepository(db.DB, logger)

	// Initialize wallet (for MVP, we'll use a mock or simplified version)
	wallet, err := wallet.NewWallet(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize wallet", zap.Error(err))
	}

	// Initialize transaction builder
	txBuilder, err := tx.NewTxBuilder(wallet, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize transaction builder", zap.Error(err))
	}

	// Initialize query client
	queryClient := query.NewQueryClient(cfg, logger)

	// Initialize service with all dependencies
	svc := service.NewService(wallet, txBuilder, queryClient, orderRepo, db, logger)

	// Create endpoints
	endpoints := endpoint.MakeEndpoints(svc)

	// Set up HTTP configuration
	httpConfig := httptransport.HTTPConfig{
		APIKey:            getEnvOrDefault("API_KEY", "default-api-key-change-in-production"),
		MaxBodySize:       1024 * 1024, // 1MB
		RequestsPerSecond: 100,
		BurstSize:         200,
		Logger:            logger,
		AllowedOrigins:    []string{"*"}, // Configure properly for production
	}

	// Set up HTTP handler with middleware
	handler := httptransport.NewHTTPHandler(endpoints, httpConfig)

	// Start server
	port := ":" + cfg.HTTPPort
	logger.Info("Starting server with security middleware",
		zap.String("port", port),
		zap.Int("max_body_size", int(httpConfig.MaxBodySize)),
		zap.Int("rate_limit", httpConfig.RequestsPerSecond))

	if err := http.ListenAndServe(port, handler); err != nil {
		logger.Fatal("Server failed", zap.Error(err))
	}
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// performHealthCheck performs a health check and exits with appropriate code
func performHealthCheck() {
	cfg := config.LoadConfig()

	// Create a simple HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Determine the health check URL
	port := cfg.HTTPPort
	if port == "" {
		port = "8080"
	}
	healthURL := fmt.Sprintf("http://localhost:%s/health", port)

	// Perform health check request
	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Printf("Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == http.StatusOK {
		fmt.Println("Health check passed")
		os.Exit(0)
	} else {
		fmt.Printf("Health check failed with status: %d\n", resp.StatusCode)
		os.Exit(1)
	}
}
