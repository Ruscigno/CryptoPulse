package reconciliation

import (
	"context"
	"fmt"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/query"
	"github.com/Ruscigno/CryptoPulse/pkg/repository"
	"go.uber.org/zap"
)

// TransactionReconciler handles reconciliation between database and chain state
type TransactionReconciler struct {
	queryClient *query.QueryClient
	orderRepo   repository.OrderRepository
	logger      *zap.Logger
}

// NewTransactionReconciler creates a new transaction reconciler
func NewTransactionReconciler(
	queryClient *query.QueryClient,
	orderRepo repository.OrderRepository,
	logger *zap.Logger,
) *TransactionReconciler {
	return &TransactionReconciler{
		queryClient: queryClient,
		orderRepo:   orderRepo,
		logger:      logger,
	}
}

// ReconciliationResult represents the result of a reconciliation
type ReconciliationResult struct {
	TotalOrders     int
	SyncedOrders    int
	FailedOrders    int
	Inconsistencies []Inconsistency
	Duration        time.Duration
	Timestamp       time.Time
}

// Inconsistency represents a data inconsistency between database and chain
type Inconsistency struct {
	OrderID     string
	Type        string // "STATUS_MISMATCH", "SIZE_MISMATCH", "MISSING_IN_DB", "MISSING_IN_CHAIN"
	DBValue     interface{}
	ChainValue  interface{}
	Description string
	Severity    string // "LOW", "MEDIUM", "HIGH"
	Resolution  string
}

// ReconcileOrders reconciles orders between database and chain state
func (tr *TransactionReconciler) ReconcileOrders(ctx context.Context, address string) (*ReconciliationResult, error) {
	tr.logger.Info("Starting order reconciliation", zap.String("address", address))

	startTime := time.Now()
	result := &ReconciliationResult{
		Timestamp: startTime,
	}

	// Get orders from chain
	chainOrders, err := tr.queryClient.GetOrders(ctx, address, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders from chain: %w", err)
	}

	if chainOrders == nil || len(chainOrders.Orders) == 0 {
		tr.logger.Info("No orders found on chain")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.TotalOrders = len(chainOrders.Orders)

	// Check each chain order against database
	for _, chainOrder := range chainOrders.Orders {
		if err := tr.reconcileSingleOrder(ctx, chainOrder, result); err != nil {
			tr.logger.Error("Failed to reconcile order",
				zap.String("order_id", chainOrder.ID),
				zap.Error(err))
			result.FailedOrders++
		} else {
			result.SyncedOrders++
		}
	}

	result.Duration = time.Since(startTime)

	tr.logger.Info("Order reconciliation completed",
		zap.Int("total_orders", result.TotalOrders),
		zap.Int("synced_orders", result.SyncedOrders),
		zap.Int("failed_orders", result.FailedOrders),
		zap.Int("inconsistencies", len(result.Inconsistencies)),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// reconcileSingleOrder reconciles a single order
func (tr *TransactionReconciler) reconcileSingleOrder(
	ctx context.Context,
	chainOrder query.OrderResponse,
	result *ReconciliationResult,
) error {
	// Try to get order from database by order ID
	dbOrder, err := tr.orderRepo.GetOrderByOrderID(ctx, chainOrder.ID)
	if err != nil {
		// Order exists on chain but not in database
		inconsistency := Inconsistency{
			OrderID:     chainOrder.ID,
			Type:        "MISSING_IN_DB",
			ChainValue:  chainOrder.Status,
			Description: "Order exists on chain but not in database",
			Severity:    "HIGH",
			Resolution:  "Create order record in database from chain data",
		}
		result.Inconsistencies = append(result.Inconsistencies, inconsistency)
		return nil
	}

	// Compare statuses
	dbStatus := string(dbOrder.Status)
	chainStatus := chainOrder.Status

	if dbStatus != chainStatus {
		inconsistency := Inconsistency{
			OrderID:     chainOrder.ID,
			Type:        "STATUS_MISMATCH",
			DBValue:     dbStatus,
			ChainValue:  chainStatus,
			Description: fmt.Sprintf("Status mismatch: DB=%s, Chain=%s", dbStatus, chainStatus),
			Severity:    "MEDIUM",
			Resolution:  "Update database status to match chain state",
		}
		result.Inconsistencies = append(result.Inconsistencies, inconsistency)

		// Auto-correct: update database to match chain
		if err := tr.updateOrderStatus(ctx, chainOrder.ID, chainStatus); err != nil {
			tr.logger.Error("Failed to auto-correct order status",
				zap.String("order_id", chainOrder.ID),
				zap.Error(err))
		}
	}

	// Compare sizes
	dbFilledSize := dbOrder.FilledSize
	chainFilledSize := parseFloat(chainOrder.Size) - parseFloat(chainOrder.RemainingSize)

	if dbFilledSize != chainFilledSize {
		inconsistency := Inconsistency{
			OrderID:     chainOrder.ID,
			Type:        "SIZE_MISMATCH",
			DBValue:     dbFilledSize,
			ChainValue:  chainFilledSize,
			Description: fmt.Sprintf("Filled size mismatch: DB=%f, Chain=%f", dbFilledSize, chainFilledSize),
			Severity:    "MEDIUM",
			Resolution:  "Update database filled size to match chain state",
		}
		result.Inconsistencies = append(result.Inconsistencies, inconsistency)

		// Auto-correct: update database to match chain
		if err := tr.updateOrderSize(ctx, chainOrder.ID, chainFilledSize); err != nil {
			tr.logger.Error("Failed to auto-correct order size",
				zap.String("order_id", chainOrder.ID),
				zap.Error(err))
		}
	}

	return nil
}

// updateOrderStatus updates order status in database
func (tr *TransactionReconciler) updateOrderStatus(ctx context.Context, orderID string, status string) error {
	// Get order from database
	order, err := tr.orderRepo.GetOrderByOrderID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	// Convert chain status to repository status
	order.Status = tr.convertStatus(status)
	order.UpdatedAt = time.Now()

	return tr.orderRepo.UpdateOrder(ctx, order)
}

// updateOrderSize updates order filled size in database
func (tr *TransactionReconciler) updateOrderSize(ctx context.Context, orderID string, filledSize float64) error {
	// Get order from database
	order, err := tr.orderRepo.GetOrderByOrderID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	order.FilledSize = filledSize
	order.RemainingSize = order.Size - filledSize
	order.UpdatedAt = time.Now()

	return tr.orderRepo.UpdateOrder(ctx, order)
}

// convertStatus converts chain status to repository status
func (tr *TransactionReconciler) convertStatus(chainStatus string) repository.OrderStatus {
	switch chainStatus {
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
		return repository.OrderStatusPending
	}
}

// parseFloat safely parses a string to float64
func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// GetInconsistencySummary returns a summary of inconsistencies by type
func (result *ReconciliationResult) GetInconsistencySummary() map[string]int {
	summary := make(map[string]int)
	for _, inc := range result.Inconsistencies {
		summary[inc.Type]++
	}
	return summary
}

// HasCriticalInconsistencies checks if there are critical inconsistencies
func (result *ReconciliationResult) HasCriticalInconsistencies() bool {
	for _, inc := range result.Inconsistencies {
		if inc.Severity == "HIGH" {
			return true
		}
	}
	return false
}
