package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
	"github.com/Ruscigno/CryptoPulse/pkg/query"
	"github.com/Ruscigno/CryptoPulse/pkg/wallet"
	"go.uber.org/zap"
)

// TestnetHelper provides utilities for testnet integration testing
type TestnetHelper struct {
	t           *testing.T
	config      config.Config
	logger      *zap.Logger
	wallet      *wallet.Wallet
	queryClient *query.QueryClient
}

// NewTestnetHelper creates a new testnet helper
func NewTestnetHelper(t *testing.T) *TestnetHelper {
	logger, _ := zap.NewDevelopment()

	// Load config from environment
	cfg := config.Config{
		Mnemonic:   os.Getenv("MNEMONIC"),
		ChainID:    os.Getenv("CHAIN_ID"),
		RPCURL:     os.Getenv("RPC_URL"),
		IndexerURL: os.Getenv("INDEXER_URL"),
	}

	// Set defaults if not provided
	if cfg.ChainID == "" {
		cfg.ChainID = "dydx-testnet-4"
	}
	if cfg.RPCURL == "" {
		cfg.RPCURL = "https://test-dydx.kingnodes.com"
	}
	if cfg.IndexerURL == "" {
		cfg.IndexerURL = "https://indexer.v4testnet.dydx.exchange/v4"
	}

	// Create wallet
	w, err := wallet.NewWallet(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create query client
	qc := query.NewQueryClient(cfg, logger)

	return &TestnetHelper{
		t:           t,
		config:      cfg,
		logger:      logger,
		wallet:      w,
		queryClient: qc,
	}
}

// GetWallet returns the test wallet
func (th *TestnetHelper) GetWallet() *wallet.Wallet {
	return th.wallet
}

// GetQueryClient returns the query client
func (th *TestnetHelper) GetQueryClient() *query.QueryClient {
	return th.queryClient
}

// GetAddress returns the wallet address
func (th *TestnetHelper) GetAddress(ctx context.Context) string {
	addr, err := th.wallet.GetAddress(ctx)
	if err != nil {
		th.t.Fatalf("Failed to get wallet address: %v", err)
	}
	return addr
}

// WaitForOrderConfirmation waits for an order to be confirmed on chain
func (th *TestnetHelper) WaitForOrderConfirmation(ctx context.Context, orderID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			order, err := th.queryClient.GetOrderByID(ctx, orderID)
			if err == nil && order != nil {
				th.logger.Info("Order confirmed on chain",
					zap.String("order_id", orderID),
					zap.String("status", order.Status))
				return nil
			}

			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for order confirmation: %s", orderID)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WaitForPositionUpdate waits for a position to be updated on chain
func (th *TestnetHelper) WaitForPositionUpdate(ctx context.Context, market string, timeout time.Duration) error {
	addr := th.GetAddress(ctx)
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			positions, err := th.queryClient.GetPositions(ctx, addr)
			if err == nil && positions != nil {
				for _, pos := range positions.Positions {
					if pos.Market == market {
						th.logger.Info("Position updated on chain",
							zap.String("market", market),
							zap.String("size", pos.Size))
						return nil
					}
				}
			}

			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for position update: %s", market)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// GetPositions retrieves current positions
func (th *TestnetHelper) GetPositions(ctx context.Context) (*query.PositionsResponse, error) {
	addr := th.GetAddress(ctx)
	return th.queryClient.GetPositions(ctx, addr)
}

// GetOrders retrieves current orders
func (th *TestnetHelper) GetOrders(ctx context.Context, status *string, market *string) (*query.OrdersResponse, error) {
	addr := th.GetAddress(ctx)
	return th.queryClient.GetOrders(ctx, addr, status, market)
}

// AssertOrderExists asserts that an order exists on chain
func (th *TestnetHelper) AssertOrderExists(ctx context.Context, orderID string) {
	order, err := th.queryClient.GetOrderByID(ctx, orderID)
	if err != nil {
		th.t.Fatalf("Failed to get order: %v", err)
	}
	if order == nil {
		th.t.Fatalf("Order not found: %s", orderID)
	}
}

// AssertOrderStatus asserts that an order has a specific status
func (th *TestnetHelper) AssertOrderStatus(ctx context.Context, orderID string, expectedStatus string) {
	order, err := th.queryClient.GetOrderByID(ctx, orderID)
	if err != nil {
		th.t.Fatalf("Failed to get order: %v", err)
	}
	if order.Status != expectedStatus {
		th.t.Fatalf("Expected order status %s, got %s", expectedStatus, order.Status)
	}
}

// AssertPositionExists asserts that a position exists
func (th *TestnetHelper) AssertPositionExists(ctx context.Context, market string) {
	positions, err := th.GetPositions(ctx)
	if err != nil {
		th.t.Fatalf("Failed to get positions: %v", err)
	}

	for _, pos := range positions.Positions {
		if pos.Market == market {
			return
		}
	}

	th.t.Fatalf("Position not found for market: %s", market)
}

// AssertPositionSize asserts that a position has a specific size
func (th *TestnetHelper) AssertPositionSize(ctx context.Context, market string, expectedSize string) {
	positions, err := th.GetPositions(ctx)
	if err != nil {
		th.t.Fatalf("Failed to get positions: %v", err)
	}

	for _, pos := range positions.Positions {
		if pos.Market == market {
			if pos.Size != expectedSize {
				th.t.Fatalf("Expected position size %s, got %s", expectedSize, pos.Size)
			}
			return
		}
	}

	th.t.Fatalf("Position not found for market: %s", market)
}

// Close closes the testnet helper
func (th *TestnetHelper) Close() {
	th.logger.Sync()
}
