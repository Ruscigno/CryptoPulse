package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CancelOrder cancels an existing order
func (s *service) CancelOrder(ctx context.Context, req CancelOrderRequest) (CancelOrderResponse, error) {
	s.logger.Info("Canceling order",
		zap.String("order_id", req.OrderID),
		zap.String("client_id", req.ClientID))

	// Find the order
	var order *repository.Order
	var err error

	if req.OrderID != "" {
		orderUUID, err := uuid.Parse(req.OrderID)
		if err != nil {
			return CancelOrderResponse{}, fmt.Errorf("invalid order ID format: %w", err)
		}
		order, err = s.orderRepo.GetOrderByID(ctx, orderUUID)
	} else if req.ClientID != "" {
		order, err = s.orderRepo.GetOrderByClientID(ctx, req.ClientID)
	} else {
		return CancelOrderResponse{}, fmt.Errorf("either order_id or client_id must be provided")
	}

	if err != nil {
		return CancelOrderResponse{}, fmt.Errorf("order not found: %w", err)
	}

	// Check if order can be cancelled
	if order.Status != repository.OrderStatusOpen && order.Status != repository.OrderStatusPartiallyFilled {
		return CancelOrderResponse{}, fmt.Errorf("order cannot be cancelled, current status: %s", order.Status)
	}

	// Cancel order via transaction builder with retry and circuit breaker
	txResponse, err := s.cancelOrderWithRetry(ctx, order.ID.String())
	if err != nil {
		s.logger.Error("Failed to cancel order after retries", zap.Error(err))
		return CancelOrderResponse{}, fmt.Errorf("failed to cancel order: %w", err)
	}

	// Update order status
	order.Status = repository.OrderStatusCancelled
	now := time.Now()
	order.CancelledAt = &now
	order.TxHash = &txResponse.TxHash
	if err := s.orderRepo.UpdateOrder(ctx, order); err != nil {
		s.logger.Error("Failed to update order after cancellation", zap.Error(err))
	}

	response := CancelOrderResponse{
		OrderID:   order.ID.String(),
		Status:    string(order.Status),
		TxHash:    txResponse.TxHash,
		Message:   "Order cancelled successfully",
		UpdatedAt: now.Format(time.RFC3339),
	}

	s.logger.Info("Order cancelled successfully",
		zap.String("order_id", response.OrderID),
		zap.String("tx_hash", response.TxHash))

	return response, nil
}

// GetPositions retrieves current positions
func (s *service) GetPositions(ctx context.Context) (PositionsResponse, error) {
	s.logger.Info("Getting positions")

	// Get wallet address
	address, err := s.wallet.GetAddress(ctx)
	if err != nil {
		return PositionsResponse{}, fmt.Errorf("failed to get wallet address: %w", err)
	}

	// Query positions from dYdX indexer with retry and circuit breaker
	positionsResp, err := s.getPositionsWithRetry(ctx, address)
	if err != nil {
		return PositionsResponse{}, fmt.Errorf("failed to get positions: %w", err)
	}

	// Convert to service response format
	positions := make([]Position, len(positionsResp.Positions))
	for i, pos := range positionsResp.Positions {
		positions[i] = Position{
			Market:        pos.Market,
			Side:          pos.Side,
			Size:          pos.Size,
			EntryPrice:    pos.EntryPrice,
			UnrealizedPnl: pos.UnrealizedPnl,
			RealizedPnl:   pos.RealizedPnl,
			CreatedAt:     pos.CreatedAt,
		}
	}

	response := PositionsResponse{
		Positions: positions,
	}

	s.logger.Info("Successfully retrieved positions", zap.Int("count", len(positions)))

	return response, nil
}

// ClosePosition closes a position by placing an opposing order
func (s *service) ClosePosition(ctx context.Context, req ClosePositionRequest) (ClosePositionResponse, error) {
	s.logger.Info("Closing position", zap.String("market", req.Market))

	// Get current positions
	positionsResp, err := s.GetPositions(ctx)
	if err != nil {
		return ClosePositionResponse{}, fmt.Errorf("failed to get positions: %w", err)
	}

	// Find the position to close
	var targetPosition *Position
	for _, pos := range positionsResp.Positions {
		if pos.Market == req.Market {
			targetPosition = &pos
			break
		}
	}

	if targetPosition == nil {
		return ClosePositionResponse{}, fmt.Errorf("no open position found for market: %s", req.Market)
	}

	// Determine opposing side
	var oppositeSide string
	if targetPosition.Side == "LONG" {
		oppositeSide = "SELL"
	} else {
		oppositeSide = "BUY"
	}

	// Parse position size
	size, err := strconv.ParseFloat(targetPosition.Size, 64)
	if err != nil {
		return ClosePositionResponse{}, fmt.Errorf("invalid position size: %w", err)
	}

	// Place market order to close position
	orderReq := OrderRequest{
		Market: req.Market,
		Side:   oppositeSide,
		Type:   "MARKET",
		Size:   size,
	}

	orderResp, err := s.PlaceOrder(ctx, orderReq)
	if err != nil {
		return ClosePositionResponse{}, fmt.Errorf("failed to place closing order: %w", err)
	}

	response := ClosePositionResponse{
		OrderID:   orderResp.OrderID,
		Market:    req.Market,
		Status:    orderResp.Status,
		TxHash:    orderResp.TxHash,
		Message:   "Position close order placed successfully",
		CreatedAt: orderResp.CreatedAt,
	}

	s.logger.Info("Position close order placed",
		zap.String("market", req.Market),
		zap.String("order_id", response.OrderID))

	return response, nil
}

// GetOrderStatus retrieves the status of a specific order
func (s *service) GetOrderStatus(ctx context.Context, orderID string) (OrderStatusResponse, error) {
	s.logger.Info("Getting order status", zap.String("order_id", orderID))

	// Parse order ID
	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return OrderStatusResponse{}, fmt.Errorf("invalid order ID format: %w", err)
	}

	// Get order from database
	order, err := s.orderRepo.GetOrderByID(ctx, orderUUID)
	if err != nil {
		return OrderStatusResponse{}, fmt.Errorf("order not found: %w", err)
	}

	// Get order history
	history, err := s.orderRepo.GetOrderHistory(ctx, orderUUID)
	if err != nil {
		s.logger.Error("Failed to get order history", zap.Error(err))
		// Continue without history
	}

	// Convert history to response format
	historyItems := make([]OrderStatusHistoryItem, len(history))
	for i, h := range history {
		historyItems[i] = OrderStatusHistoryItem{
			OldStatus: func() string {
				if h.OldStatus != nil {
					return string(*h.OldStatus)
				}
				return ""
			}(),
			NewStatus: string(h.NewStatus),
			Timestamp: h.CreatedAt.Format(time.RFC3339),
			Reason: func() string {
				if h.Reason != nil {
					return *h.Reason
				}
				return ""
			}(),
		}
	}

	response := OrderStatusResponse{
		OrderID:       order.ID.String(),
		ClientID:      order.ClientID,
		Status:        string(order.Status),
		Market:        order.Market,
		Side:          string(order.Side),
		Size:          fmt.Sprintf("%.8f", order.Size),
		FilledSize:    fmt.Sprintf("%.8f", order.FilledSize),
		RemainingSize: fmt.Sprintf("%.8f", order.RemainingSize),
		Price: func() *string {
			if order.Price != nil {
				price := fmt.Sprintf("%.8f", *order.Price)
				return &price
			}
			return nil
		}(),
		CreatedAt: order.CreatedAt.Format(time.RFC3339),
		UpdatedAt: order.UpdatedAt.Format(time.RFC3339),
		History:   historyItems,
	}

	s.logger.Info("Successfully retrieved order status", zap.String("order_id", orderID))

	return response, nil
}

// GetOrderHistory retrieves order history based on filters
func (s *service) GetOrderHistory(ctx context.Context, req OrderHistoryRequest) (OrderHistoryResponse, error) {
	s.logger.Info("Getting order history",
		zap.Any("market", req.Market),
		zap.Any("status", req.Status),
		zap.Int("limit", req.Limit))

	// Set default limit
	if req.Limit <= 0 {
		req.Limit = 50
	}

	// Build filters
	filters := repository.OrderFilters{
		Market: req.Market,
		Limit:  req.Limit,
		Offset: req.Offset,
	}

	if req.Status != nil {
		status := repository.OrderStatus(*req.Status)
		filters.Status = &status
	}

	// Get orders from database
	orders, err := s.orderRepo.ListOrders(ctx, filters)
	if err != nil {
		return OrderHistoryResponse{}, fmt.Errorf("failed to get order history: %w", err)
	}

	// Convert to response format
	orderResponses := make([]OrderStatusResponse, len(orders))
	for i, order := range orders {
		orderResponses[i] = OrderStatusResponse{
			OrderID:       order.ID.String(),
			ClientID:      order.ClientID,
			Status:        string(order.Status),
			Market:        order.Market,
			Side:          string(order.Side),
			Size:          fmt.Sprintf("%.8f", order.Size),
			FilledSize:    fmt.Sprintf("%.8f", order.FilledSize),
			RemainingSize: fmt.Sprintf("%.8f", order.RemainingSize),
			Price: func() *string {
				if order.Price != nil {
					price := fmt.Sprintf("%.8f", *order.Price)
					return &price
				}
				return nil
			}(),
			CreatedAt: order.CreatedAt.Format(time.RFC3339),
			UpdatedAt: order.UpdatedAt.Format(time.RFC3339),
		}
	}

	response := OrderHistoryResponse{
		Orders: orderResponses,
		Total:  len(orderResponses),
	}

	s.logger.Info("Successfully retrieved order history", zap.Int("count", len(orderResponses)))

	return response, nil
}
