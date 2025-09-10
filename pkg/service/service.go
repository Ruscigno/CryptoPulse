package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/database"
	"github.com/Ruscigno/CryptoPulse/pkg/query"
	"github.com/Ruscigno/CryptoPulse/pkg/repository"
	"github.com/Ruscigno/CryptoPulse/pkg/retry"
	"github.com/Ruscigno/CryptoPulse/pkg/tx"
	"github.com/Ruscigno/CryptoPulse/pkg/wallet"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// OrderRequest defines the input for placing an order
type OrderRequest struct {
	Market       string   `json:"market" validate:"required"`
	Side         string   `json:"side" validate:"required,oneof=BUY SELL"`
	Type         string   `json:"type" validate:"required,oneof=MARKET LIMIT"`
	Size         float64  `json:"size" validate:"required,gt=0"`
	Price        *float64 `json:"price,omitempty"`
	TimeInForce  string   `json:"timeInForce,omitempty" validate:"omitempty,oneof=GTT FOK IOC"`
	GoodTilBlock *uint32  `json:"goodTilBlock,omitempty"`
}

// OrderResponse defines the response for an order operation
type OrderResponse struct {
	OrderID   string `json:"orderId"`
	ClientID  string `json:"clientId"`
	Status    string `json:"status"`
	TxHash    string `json:"txHash,omitempty"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// CancelOrderRequest defines the input for canceling an order
type CancelOrderRequest struct {
	OrderID  string `json:"orderId,omitempty"`
	ClientID string `json:"clientId,omitempty"`
}

// CancelOrderResponse defines the response for a cancel order operation
type CancelOrderResponse struct {
	OrderID   string `json:"orderId"`
	Status    string `json:"status"`
	TxHash    string `json:"txHash,omitempty"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
}

// PositionsResponse defines the response for getting positions
type PositionsResponse struct {
	Positions []Position `json:"positions"`
}

// Position represents a trading position
type Position struct {
	Market        string `json:"market"`
	Side          string `json:"side"`
	Size          string `json:"size"`
	EntryPrice    string `json:"entryPrice"`
	UnrealizedPnl string `json:"unrealizedPnl"`
	RealizedPnl   string `json:"realizedPnl"`
	CreatedAt     string `json:"createdAt"`
}

// ClosePositionRequest defines the input for closing a position
type ClosePositionRequest struct {
	Market string `json:"market" validate:"required"`
}

// ClosePositionResponse defines the response for closing a position
type ClosePositionResponse struct {
	OrderID   string `json:"orderId"`
	Market    string `json:"market"`
	Status    string `json:"status"`
	TxHash    string `json:"txHash,omitempty"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
}

// OrderStatusResponse defines the response for getting order status
type OrderStatusResponse struct {
	OrderID       string                   `json:"orderId"`
	ClientID      string                   `json:"clientId"`
	Status        string                   `json:"status"`
	Market        string                   `json:"market"`
	Side          string                   `json:"side"`
	Size          string                   `json:"size"`
	FilledSize    string                   `json:"filledSize"`
	RemainingSize string                   `json:"remainingSize"`
	Price         *string                  `json:"price,omitempty"`
	CreatedAt     string                   `json:"createdAt"`
	UpdatedAt     string                   `json:"updatedAt"`
	History       []OrderStatusHistoryItem `json:"history,omitempty"`
}

// OrderStatusHistoryItem represents a single status change
type OrderStatusHistoryItem struct {
	OldStatus string `json:"oldStatus,omitempty"`
	NewStatus string `json:"newStatus"`
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason,omitempty"`
}

// OrderHistoryRequest defines the input for getting order history
type OrderHistoryRequest struct {
	Market *string `json:"market,omitempty"`
	Status *string `json:"status,omitempty"`
	Limit  int     `json:"limit,omitempty"`
	Offset int     `json:"offset,omitempty"`
}

// OrderHistoryResponse defines the response for getting order history
type OrderHistoryResponse struct {
	Orders []OrderStatusResponse `json:"orders"`
	Total  int                   `json:"total"`
}

// Service defines the order routing service interface
type Service interface {
	PlaceOrder(ctx context.Context, req OrderRequest) (OrderResponse, error)
	CancelOrder(ctx context.Context, req CancelOrderRequest) (CancelOrderResponse, error)
	GetPositions(ctx context.Context) (PositionsResponse, error)
	ClosePosition(ctx context.Context, req ClosePositionRequest) (ClosePositionResponse, error)
	GetOrderStatus(ctx context.Context, orderID string) (OrderStatusResponse, error)
	GetOrderHistory(ctx context.Context, req OrderHistoryRequest) (OrderHistoryResponse, error)
}

// service implements the Service interface
type service struct {
	wallet         *wallet.Wallet
	txBuilder      *tx.TxBuilder
	queryClient    *query.QueryClient
	orderRepo      repository.OrderRepository
	db             *database.DB
	logger         *zap.Logger
	circuitBreaker *retry.CircuitBreakerManager
	retryConfig    retry.RetryConfig
}

// NewService creates a new Service instance with all dependencies
func NewService(
	wallet *wallet.Wallet,
	txBuilder *tx.TxBuilder,
	queryClient *query.QueryClient,
	orderRepo repository.OrderRepository,
	db *database.DB,
	logger *zap.Logger,
) Service {
	// Initialize circuit breaker manager
	circuitBreakerManager := retry.NewCircuitBreakerManager()

	// Configure retry settings
	retryConfig := retry.RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		Logger:        logger,
	}

	return &service{
		wallet:         wallet,
		txBuilder:      txBuilder,
		queryClient:    queryClient,
		orderRepo:      orderRepo,
		db:             db,
		logger:         logger,
		circuitBreaker: circuitBreakerManager,
		retryConfig:    retryConfig,
	}
}

// PlaceOrder places a new order
func (s *service) PlaceOrder(ctx context.Context, req OrderRequest) (OrderResponse, error) {
	s.logger.Info("Placing order",
		zap.String("market", req.Market),
		zap.String("side", req.Side),
		zap.String("type", req.Type),
		zap.Float64("size", req.Size))

	// Validate request
	if err := s.validateOrderRequest(req); err != nil {
		return OrderResponse{}, fmt.Errorf("invalid order request: %w", err)
	}

	// Generate client ID
	clientID := uuid.New().String()

	// Create order in database
	order := &repository.Order{
		ID:            uuid.New(),
		ClientID:      clientID,
		Market:        req.Market,
		Side:          repository.OrderSide(req.Side),
		Type:          repository.OrderType(req.Type),
		Size:          req.Size,
		Price:         req.Price,
		Status:        repository.OrderStatusPending,
		FilledSize:    0,
		RemainingSize: req.Size,
		TimeInForce:   repository.TimeInForce(getTimeInForce(req.TimeInForce)),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.orderRepo.CreateOrder(ctx, order); err != nil {
		s.logger.Error("Failed to create order in database", zap.Error(err))
		return OrderResponse{}, fmt.Errorf("failed to create order: %w", err)
	}

	// Build transaction parameters
	txParams := tx.OrderParams{
		Market:       req.Market,
		Side:         req.Side,
		Type:         req.Type,
		Size:         req.Size,
		Price:        req.Price,
		TimeInForce:  getTimeInForce(req.TimeInForce),
		GoodTilBlock: req.GoodTilBlock,
	}

	// Place order via transaction builder with retry and circuit breaker
	txResponse, err := s.placeOrderWithRetry(ctx, txParams)
	if err != nil {
		// Update order status to rejected
		order.Status = repository.OrderStatusRejected
		errMsg := err.Error()
		order.ErrorMessage = &errMsg
		s.orderRepo.UpdateOrder(ctx, order)

		s.logger.Error("Failed to place order after retries", zap.Error(err))
		return OrderResponse{}, fmt.Errorf("failed to place order: %w", err)
	}

	// Update order with transaction details
	order.TxHash = &txResponse.TxHash
	order.Status = repository.OrderStatusOpen
	order.PlacedAt = &txResponse.Timestamp
	if err := s.orderRepo.UpdateOrder(ctx, order); err != nil {
		s.logger.Error("Failed to update order after placement", zap.Error(err))
	}

	response := OrderResponse{
		OrderID:   order.ID.String(),
		ClientID:  clientID,
		Status:    string(order.Status),
		TxHash:    txResponse.TxHash,
		Message:   "Order placed successfully",
		CreatedAt: order.CreatedAt.Format(time.RFC3339),
	}

	s.logger.Info("Order placed successfully",
		zap.String("order_id", response.OrderID),
		zap.String("tx_hash", response.TxHash))

	return response, nil
}

// validateOrderRequest validates the order request
func (s *service) validateOrderRequest(req OrderRequest) error {
	if req.Market == "" {
		return fmt.Errorf("market is required")
	}
	if req.Side != "BUY" && req.Side != "SELL" {
		return fmt.Errorf("side must be BUY or SELL")
	}
	if req.Type != "MARKET" && req.Type != "LIMIT" {
		return fmt.Errorf("type must be MARKET or LIMIT")
	}
	if req.Size <= 0 {
		return fmt.Errorf("size must be greater than 0")
	}
	if req.Type == "LIMIT" && req.Price == nil {
		return fmt.Errorf("price is required for LIMIT orders")
	}
	if req.Type == "MARKET" && req.Price != nil {
		return fmt.Errorf("price should not be specified for MARKET orders")
	}
	return nil
}

// getTimeInForce returns the time in force, defaulting to GTT
func getTimeInForce(tif string) string {
	if tif == "" {
		return "GTT"
	}
	return tif
}

// isRetryableError determines if an error should be retried
func (s *service) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Retry on network errors, timeouts, and temporary failures
	retryablePatterns := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"no such host",
		"context deadline exceeded",
		"EOF",
		"connection reset by peer",
		"service unavailable",
		"internal server error",
		"bad gateway",
		"gateway timeout",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// placeOrderWithRetry places an order with retry and circuit breaker logic
func (s *service) placeOrderWithRetry(ctx context.Context, txParams tx.OrderParams) (*tx.TxResponse, error) {
	// Get or create circuit breaker for transaction operations
	cbConfig := retry.DefaultCircuitBreakerConfig("dydx-transactions")
	cbConfig.Logger = s.logger
	cb := s.circuitBreaker.GetOrCreate("dydx-transactions", cbConfig)

	// Create retry function that returns a result
	retryFunc := func() (interface{}, error) {
		return s.txBuilder.PlaceOrder(ctx, txParams)
	}

	// Execute with circuit breaker
	result, err := cb.ExecuteWithResult(ctx, func() (interface{}, error) {
		// Execute with retry logic
		return retry.RetryWithResultFunc(ctx, s.retryConfig, retryFunc)
	})

	if err != nil {
		s.logger.Error("Failed to place order with retry and circuit breaker",
			zap.Error(err),
			zap.String("market", txParams.Market),
			zap.String("side", txParams.Side))
		return nil, err
	}

	// Type assert the result
	txResponse, ok := result.(*tx.TxResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from transaction builder")
	}

	return txResponse, nil
}

// cancelOrderWithRetry cancels an order with retry and circuit breaker logic
func (s *service) cancelOrderWithRetry(ctx context.Context, orderID string) (*tx.TxResponse, error) {
	// Get or create circuit breaker for transaction operations
	cbConfig := retry.DefaultCircuitBreakerConfig("dydx-transactions")
	cbConfig.Logger = s.logger
	cb := s.circuitBreaker.GetOrCreate("dydx-transactions", cbConfig)

	// Create retry function that returns a result
	retryFunc := func() (interface{}, error) {
		return s.txBuilder.CancelOrder(ctx, orderID)
	}

	// Execute with circuit breaker
	result, err := cb.ExecuteWithResult(ctx, func() (interface{}, error) {
		// Execute with retry logic
		return retry.RetryWithResultFunc(ctx, s.retryConfig, retryFunc)
	})

	if err != nil {
		s.logger.Error("Failed to cancel order with retry and circuit breaker",
			zap.Error(err),
			zap.String("order_id", orderID))
		return nil, err
	}

	// Type assert the result
	txResponse, ok := result.(*tx.TxResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from transaction builder")
	}

	return txResponse, nil
}

// getPositionsWithRetry queries positions with retry and circuit breaker logic
func (s *service) getPositionsWithRetry(ctx context.Context, address string) (*query.PositionsResponse, error) {
	// Get or create circuit breaker for query operations
	cbConfig := retry.DefaultCircuitBreakerConfig("dydx-queries")
	cbConfig.Logger = s.logger
	cb := s.circuitBreaker.GetOrCreate("dydx-queries", cbConfig)

	// Create retry function that returns a result
	retryFunc := func() (interface{}, error) {
		return s.queryClient.GetPositions(ctx, address)
	}

	// Execute with circuit breaker
	result, err := cb.ExecuteWithResult(ctx, func() (interface{}, error) {
		// Execute with retry logic
		return retry.RetryWithResultFunc(ctx, s.retryConfig, retryFunc)
	})

	if err != nil {
		s.logger.Error("Failed to get positions with retry and circuit breaker",
			zap.Error(err),
			zap.String("address", address))
		return nil, err
	}

	// Type assert the result
	positionsResp, ok := result.(*query.PositionsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from query client")
	}

	return positionsResp, nil
}
