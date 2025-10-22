package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/query"
	"github.com/Ruscigno/CryptoPulse/pkg/repository"
	"go.uber.org/zap"
)

// OrderSyncService handles synchronization of order status from dYdX Indexer API
type OrderSyncService struct {
	queryClient  *query.QueryClient
	orderRepo    repository.OrderRepository
	logger       *zap.Logger
	pollInterval time.Duration
	stopChan     chan struct{}
	ticker       *time.Ticker
}

// NewOrderSyncService creates a new order sync service
func NewOrderSyncService(
	queryClient *query.QueryClient,
	orderRepo repository.OrderRepository,
	logger *zap.Logger,
	pollInterval time.Duration,
) *OrderSyncService {
	return &OrderSyncService{
		queryClient:  queryClient,
		orderRepo:    orderRepo,
		logger:       logger,
		pollInterval: pollInterval,
		stopChan:     make(chan struct{}),
	}
}

// Start begins the background order synchronization service
func (oss *OrderSyncService) Start(ctx context.Context, address string) error {
	oss.logger.Info("Starting order sync service",
		zap.String("address", address),
		zap.Duration("poll_interval", oss.pollInterval))

	oss.ticker = time.NewTicker(oss.pollInterval)

	go func() {
		// Sync immediately on start
		if err := oss.syncOrders(ctx, address); err != nil {
			oss.logger.Error("Initial order sync failed", zap.Error(err))
		}

		// Then sync on interval
		for {
			select {
			case <-oss.ticker.C:
				if err := oss.syncOrders(ctx, address); err != nil {
					oss.logger.Error("Order sync failed", zap.Error(err))
				}
			case <-oss.stopChan:
				oss.logger.Info("Order sync service stopped")
				return
			case <-ctx.Done():
				oss.logger.Info("Order sync service context cancelled")
				return
			}
		}
	}()

	return nil
}

// Stop stops the background order synchronization service
func (oss *OrderSyncService) Stop() {
	oss.logger.Info("Stopping order sync service")
	if oss.ticker != nil {
		oss.ticker.Stop()
	}
	close(oss.stopChan)
}

// syncOrders synchronizes orders from Indexer API to database
func (oss *OrderSyncService) syncOrders(ctx context.Context, address string) error {
	oss.logger.Debug("Syncing orders from Indexer API", zap.String("address", address))

	// Query all orders from Indexer API
	ordersResp, err := oss.queryClient.GetOrders(ctx, address, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to query orders from Indexer: %w", err)
	}

	if ordersResp == nil || len(ordersResp.Orders) == 0 {
		oss.logger.Debug("No orders found in Indexer API")
		return nil
	}

	// Process each order
	for _, indexerOrder := range ordersResp.Orders {
		if err := oss.syncSingleOrder(ctx, indexerOrder); err != nil {
			oss.logger.Error("Failed to sync order",
				zap.String("order_id", indexerOrder.ID),
				zap.Error(err))
			// Continue syncing other orders
			continue
		}
	}

	oss.logger.Debug("Order sync completed",
		zap.Int("orders_synced", len(ordersResp.Orders)))

	return nil
}

// syncSingleOrder synchronizes a single order from Indexer API to database
func (oss *OrderSyncService) syncSingleOrder(ctx context.Context, indexerOrder query.OrderResponse) error {
	// Convert Indexer order status to repository status
	status := oss.convertOrderStatus(indexerOrder.Status)

	// Parse sizes
	filledSize := parseFloat(indexerOrder.Size) - parseFloat(indexerOrder.RemainingSize)
	remainingSize := parseFloat(indexerOrder.RemainingSize)

	// Get order by order ID from database
	order, err := oss.orderRepo.GetOrderByOrderID(ctx, indexerOrder.ID)
	if err != nil {
		oss.logger.Warn("Order not found in database, skipping sync",
			zap.String("order_id", indexerOrder.ID))
		return nil
	}

	// Update order fields
	order.Status = status
	order.FilledSize = filledSize
	order.RemainingSize = remainingSize
	order.UpdatedAt = time.Now()

	if err := oss.orderRepo.UpdateOrder(ctx, order); err != nil {
		return fmt.Errorf("failed to update order in database: %w", err)
	}

	oss.logger.Debug("Order synced",
		zap.String("order_id", indexerOrder.ID),
		zap.String("status", string(status)),
		zap.Float64("filled_size", filledSize),
		zap.Float64("remaining_size", remainingSize))

	return nil
}

// convertOrderStatus converts Indexer API order status to repository status
func (oss *OrderSyncService) convertOrderStatus(indexerStatus string) repository.OrderStatus {
	switch indexerStatus {
	case "OPEN":
		return repository.OrderStatusOpen
	case "FILLED":
		return repository.OrderStatusFilled
	case "PARTIALLY_FILLED":
		return repository.OrderStatusPartiallyFilled
	case "CANCELLED":
		return repository.OrderStatusCancelled
	case "EXPIRED":
		return repository.OrderStatusExpired
	default:
		oss.logger.Warn("Unknown order status from Indexer",
			zap.String("status", indexerStatus))
		return repository.OrderStatusPending
	}
}

// parseFloat safely parses a string to float64
func parseFloat(s string) float64 {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	if err != nil {
		// Return 0 if parsing fails
		return 0
	}
	return f
}

// SyncOrdersOnce performs a single synchronization of orders
func (oss *OrderSyncService) SyncOrdersOnce(ctx context.Context, address string) error {
	return oss.syncOrders(ctx, address)
}
