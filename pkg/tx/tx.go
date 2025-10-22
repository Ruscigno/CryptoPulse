package tx

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
	"github.com/Ruscigno/CryptoPulse/pkg/query"
	"github.com/Ruscigno/CryptoPulse/pkg/wallet"
	"go.uber.org/zap"
)

// TxBuilder handles transaction construction and broadcasting for dYdX
type TxBuilder struct {
	wallet         *wallet.Wallet
	config         config.Config
	logger         *zap.Logger
	chainID        string
	gasPrice       string
	gasAdjustment  float64
	factory        *TransactionFactory
	messageBuilder *MessageBuilder
	broadcaster    *Broadcaster
	marketCache    *query.MarketCache
	subaccountID   uint32
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
func NewTxBuilder(
	w *wallet.Wallet,
	cfg config.Config,
	logger *zap.Logger,
	queryClient *query.QueryClient,
) (*TxBuilder, error) {
	gasPrice := getEnvString("GAS_PRICE", "0.025udydx")
	gasLimit := uint64(getEnvFloat("GAS_LIMIT", 200000))

	// Create transaction factory
	factory := NewTransactionFactory(
		nil, // codec will be set up in production
		cfg.ChainID,
		gasPrice,
		gasLimit,
		logger,
		w,
	)

	// Create message builder
	messageBuilder := NewMessageBuilder(logger)

	// Create broadcaster
	broadcaster := NewBroadcaster(
		cfg.RPCURL,
		30*time.Second,
		logger,
	)

	// Create market cache
	marketCache := query.NewMarketCache(queryClient, 5*time.Minute, logger)

	return &TxBuilder{
		wallet:         w,
		config:         cfg,
		logger:         logger,
		chainID:        cfg.ChainID,
		gasPrice:       gasPrice,
		gasAdjustment:  getEnvFloat("GAS_ADJUSTMENT", 1.5),
		factory:        factory,
		messageBuilder: messageBuilder,
		broadcaster:    broadcaster,
		marketCache:    marketCache,
		subaccountID:   0, // MVP uses subaccount 0
	}, nil
}

// PlaceOrder builds and broadcasts a place order transaction
func (t *TxBuilder) PlaceOrder(ctx context.Context, params OrderParams) (*TxResponse, error) {
	t.logger.Info("Building place order transaction",
		zap.String("market", params.Market),
		zap.String("side", params.Side),
		zap.Float64("size", params.Size))

	// 1. Fetch market configuration
	market, err := t.marketCache.GetMarket(ctx, params.Market)
	if err != nil {
		t.logger.Error("Failed to fetch market configuration",
			zap.String("market", params.Market),
			zap.Error(err))
		return nil, fmt.Errorf("failed to fetch market configuration: %w", err)
	}

	// 2. Quantize size and price
	quantums, err := t.messageBuilder.QuantizeSize(params.Size, market.AtomicResolution)
	if err != nil {
		t.logger.Error("Failed to quantize size", zap.Error(err))
		return nil, err
	}

	if params.Price == nil {
		return nil, fmt.Errorf("price is required for order placement")
	}

	subticks, err := t.messageBuilder.QuantizePrice(*params.Price, market.SubticksPerTick)
	if err != nil {
		t.logger.Error("Failed to quantize price", zap.Error(err))
		return nil, err
	}

	// 3. Validate quantization
	err = t.messageBuilder.ValidateQuantization(quantums, subticks, market.StepBaseQuantums)
	if err != nil {
		t.logger.Error("Quantization validation failed", zap.Error(err))
		return nil, err
	}

	// 4. Generate client order ID
	clientOrderID := t.messageBuilder.GenerateClientOrderID()

	// 5. Build message
	senderAddress, err := t.wallet.GetAddress(ctx)
	if err != nil {
		t.logger.Error("Failed to get wallet address", zap.Error(err))
		return nil, err
	}

	goodTilBlock := *params.GoodTilBlock
	if goodTilBlock == 0 {
		goodTilBlock = 100 // Default to 100 blocks
	}

	msg, err := t.messageBuilder.BuildPlaceOrderMsg(
		senderAddress,
		params.Market,
		params.Side,
		quantums,
		subticks,
		goodTilBlock,
		params.TimeInForce,
		clientOrderID,
		t.subaccountID,
	)
	if err != nil {
		t.logger.Error("Failed to build message", zap.Error(err))
		return nil, err
	}

	t.logger.Debug("Order message built successfully",
		zap.String("market", params.Market),
		zap.Uint64("quantums", quantums),
		zap.Uint64("subticks", subticks))

	// 6. Broadcast transaction (mock for now)
	// Convert message to bytes for broadcasting
	msgBytes := []byte(fmt.Sprintf("%+v", msg))
	response, err := t.broadcaster.BroadcastAndWait(ctx, msgBytes)
	if err != nil {
		t.logger.Error("Failed to broadcast transaction", zap.Error(err))
		return nil, err
	}

	t.logger.Info("Place order transaction broadcasted",
		zap.String("tx_hash", response.TxHash),
		zap.Uint32("code", response.Code))

	return response, nil
}

// CancelOrder builds and broadcasts a cancel order transaction
func (t *TxBuilder) CancelOrder(ctx context.Context, orderID string) (*TxResponse, error) {
	t.logger.Info("Building cancel order transaction", zap.String("order_id", orderID))

	// Get sender address
	senderAddress, err := t.wallet.GetAddress(ctx)
	if err != nil {
		t.logger.Error("Failed to get wallet address", zap.Error(err))
		return nil, err
	}

	// Parse order ID (format: market:clientOrderID)
	// For now, use a simple parsing approach
	// In production, this would be more sophisticated

	// Build cancel message
	// Note: This is a simplified version. In production, you would need to:
	// 1. Parse the order ID correctly
	// 2. Get the market from the order ID
	// 3. Build the proper MsgCancelOrder with correct OrderId structure

	t.logger.Debug("Cancel order message built",
		zap.String("sender", senderAddress),
		zap.String("order_id", orderID))

	// Broadcast transaction (mock for now)
	response, err := t.broadcaster.BroadcastAndWait(ctx, []byte(orderID))
	if err != nil {
		t.logger.Error("Failed to broadcast cancel transaction", zap.Error(err))
		return nil, err
	}

	t.logger.Info("Cancel order transaction broadcasted",
		zap.String("tx_hash", response.TxHash),
		zap.Uint32("code", response.Code))

	return response, nil
}

// QuantizeSize converts a size to quantums (dYdX internal representation)
func (t *TxBuilder) QuantizeSize(ctx context.Context, size float64, market string) (*big.Int, error) {
	// Fetch market configuration
	marketMeta, err := t.marketCache.GetMarket(ctx, market)
	if err != nil {
		t.logger.Error("Failed to fetch market configuration",
			zap.String("market", market),
			zap.Error(err))
		return nil, err
	}

	// Use message builder to quantize
	quantums, err := t.messageBuilder.QuantizeSize(size, marketMeta.AtomicResolution)
	if err != nil {
		return nil, err
	}

	return big.NewInt(int64(quantums)), nil
}

// QuantizePrice converts a price to subticks (dYdX internal representation)
func (t *TxBuilder) QuantizePrice(ctx context.Context, price float64, market string) (*big.Int, error) {
	// Fetch market configuration
	marketMeta, err := t.marketCache.GetMarket(ctx, market)
	if err != nil {
		t.logger.Error("Failed to fetch market configuration",
			zap.String("market", market),
			zap.Error(err))
		return nil, err
	}

	// Use message builder to quantize
	subticks, err := t.messageBuilder.QuantizePrice(price, marketMeta.SubticksPerTick)
	if err != nil {
		return nil, err
	}

	return big.NewInt(int64(subticks)), nil
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
