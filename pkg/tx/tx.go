package tx

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
	"github.com/Ruscigno/CryptoPulse/pkg/wallet"
	"go.uber.org/zap"
)

// TxBuilder handles transaction construction and broadcasting for dYdX
type TxBuilder struct {
	wallet        *wallet.Wallet
	config        config.Config
	logger        *zap.Logger
	chainID       string
	gasPrice      string
	gasAdjustment float64
}

// TxBuilderConfig holds transaction builder configuration
type TxBuilderConfig struct {
	ChainID       string
	GasPrice      string
	GasAdjustment float64
	Timeout       time.Duration
}

// OrderParams represents parameters for placing an order
type OrderParams struct {
	Market       string
	Side         string // "BUY" or "SELL"
	Type         string // "MARKET", "LIMIT", etc.
	Size         float64
	Price        *float64 // nil for market orders
	TimeInForce  string   // "GTT", "FOK", "IOC"
	GoodTilBlock *uint32
}

// TxResponse represents the response from a transaction broadcast
type TxResponse struct {
	TxHash    string
	Code      uint32
	RawLog    string
	GasUsed   int64
	GasWanted int64
	Height    int64
	Timestamp time.Time
}

// NewTxBuilder creates a new TxBuilder instance
func NewTxBuilder(w *wallet.Wallet, cfg config.Config, logger *zap.Logger) (*TxBuilder, error) {
	return &TxBuilder{
		wallet:        w,
		config:        cfg,
		logger:        logger,
		chainID:       cfg.ChainID,
		gasPrice:      getEnvString("GAS_PRICE", "0.025udydx"),
		gasAdjustment: getEnvFloat("GAS_ADJUSTMENT", 1.5),
	}, nil
}

// PlaceOrder builds and broadcasts a place order transaction
func (t *TxBuilder) PlaceOrder(ctx context.Context, params OrderParams) (*TxResponse, error) {
	t.logger.Info("Building place order transaction",
		zap.String("market", params.Market),
		zap.String("side", params.Side),
		zap.Float64("size", params.Size))

	// For MVP, return a mock response
	// TODO: Implement actual dYdX transaction building and broadcasting
	response := &TxResponse{
		TxHash:    fmt.Sprintf("mock-tx-%d", time.Now().Unix()),
		Code:      0, // Success
		RawLog:    "Mock transaction successful",
		GasUsed:   50000,
		GasWanted: 60000,
		Height:    12345,
		Timestamp: time.Now(),
	}

	t.logger.Info("Place order transaction broadcasted (mock)",
		zap.String("tx_hash", response.TxHash),
		zap.Uint32("code", response.Code))

	return response, nil
}

// CancelOrder builds and broadcasts a cancel order transaction
func (t *TxBuilder) CancelOrder(ctx context.Context, orderID string) (*TxResponse, error) {
	t.logger.Info("Building cancel order transaction", zap.String("order_id", orderID))

	// For MVP, return a mock response
	response := &TxResponse{
		TxHash:    fmt.Sprintf("mock-cancel-tx-%d", time.Now().Unix()),
		Code:      0,
		RawLog:    "Mock cancel transaction successful",
		GasUsed:   30000,
		GasWanted: 40000,
		Height:    12346,
		Timestamp: time.Now(),
	}

	t.logger.Info("Cancel order transaction broadcasted (mock)",
		zap.String("tx_hash", response.TxHash))

	return response, nil
}

// QuantizeSize converts a size to quantums (dYdX internal representation)
func (t *TxBuilder) QuantizeSize(size float64, market string) (*big.Int, error) {
	// This is a placeholder implementation
	// In a real implementation, you would fetch market configuration
	// and apply the correct quantization logic
	quantums := big.NewInt(int64(size * 1000000)) // Mock: 6 decimal places
	return quantums, nil
}

// QuantizePrice converts a price to subticks (dYdX internal representation)
func (t *TxBuilder) QuantizePrice(price float64, market string) (*big.Int, error) {
	// This is a placeholder implementation
	// In a real implementation, you would fetch market configuration
	// and apply the correct quantization logic
	subticks := big.NewInt(int64(price * 1000000)) // Mock: 6 decimal places
	return subticks, nil
}

// Helper functions
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}
