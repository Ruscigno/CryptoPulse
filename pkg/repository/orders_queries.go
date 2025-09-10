package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ListOrders retrieves orders based on filters
func (r *orderRepository) ListOrders(ctx context.Context, filters OrderFilters) ([]*Order, error) {
	query := `SELECT * FROM orders WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filters.Market != nil {
		query += fmt.Sprintf(" AND market = $%d", argIndex)
		args = append(args, *filters.Market)
		argIndex++
	}

	if filters.Side != nil {
		query += fmt.Sprintf(" AND side = $%d", argIndex)
		args = append(args, *filters.Side)
		argIndex++
	}

	if filters.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, *filters.Type)
		argIndex++
	}

	if filters.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *filters.Status)
		argIndex++
	}

	if filters.CreatedAfter != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, *filters.CreatedAfter)
		argIndex++
	}

	if filters.CreatedBefore != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, *filters.CreatedBefore)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filters.Limit)
		argIndex++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filters.Offset)
	}

	var orders []*Order
	err := r.db.SelectContext(ctx, &orders, query, args...)
	if err != nil {
		r.logger.Error("Failed to list orders", zap.Error(err))
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}

	return orders, nil
}

// ListActiveOrders retrieves active orders for a market
func (r *orderRepository) ListActiveOrders(ctx context.Context, market string) ([]*Order, error) {
	query := `SELECT * FROM active_orders WHERE market = $1 ORDER BY created_at DESC`

	var orders []*Order
	err := r.db.SelectContext(ctx, &orders, query, market)
	if err != nil {
		r.logger.Error("Failed to list active orders", zap.Error(err), zap.String("market", market))
		return nil, fmt.Errorf("failed to list active orders: %w", err)
	}

	return orders, nil
}

// ListOrdersByStatus retrieves orders by status
func (r *orderRepository) ListOrdersByStatus(ctx context.Context, status OrderStatus, limit int) ([]*Order, error) {
	query := `SELECT * FROM orders WHERE status = $1 ORDER BY created_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var orders []*Order
	err := r.db.SelectContext(ctx, &orders, query, status)
	if err != nil {
		r.logger.Error("Failed to list orders by status", zap.Error(err), zap.String("status", string(status)))
		return nil, fmt.Errorf("failed to list orders by status: %w", err)
	}

	return orders, nil
}

// CountOrdersByStatus counts orders by status
func (r *orderRepository) CountOrdersByStatus(ctx context.Context, status OrderStatus) (int, error) {
	query := `SELECT COUNT(*) FROM orders WHERE status = $1`

	var count int
	err := r.db.GetContext(ctx, &count, query, status)
	if err != nil {
		r.logger.Error("Failed to count orders by status", zap.Error(err), zap.String("status", string(status)))
		return 0, fmt.Errorf("failed to count orders by status: %w", err)
	}

	return count, nil
}

// GetOrderHistory retrieves the status history for an order
func (r *orderRepository) GetOrderHistory(ctx context.Context, orderID uuid.UUID) ([]*OrderStatusHistory, error) {
	query := `SELECT * FROM order_status_history WHERE order_id = $1 ORDER BY created_at ASC`

	var history []*OrderStatusHistory
	err := r.db.SelectContext(ctx, &history, query, orderID)
	if err != nil {
		r.logger.Error("Failed to get order history", zap.Error(err), zap.String("order_id", orderID.String()))
		return nil, fmt.Errorf("failed to get order history: %w", err)
	}

	return history, nil
}

// CreateOrderStatusHistory creates a new order status history entry
func (r *orderRepository) CreateOrderStatusHistory(ctx context.Context, history *OrderStatusHistory) error {
	if history.ID == uuid.Nil {
		history.ID = uuid.New()
	}

	query := `
		INSERT INTO order_status_history (
			id, order_id, old_status, new_status, filled_size, remaining_size,
			fill_price, tx_hash, block_height, reason
		) VALUES (
			:id, :order_id, :old_status, :new_status, :filled_size, :remaining_size,
			:fill_price, :tx_hash, :block_height, :reason
		)`

	_, err := r.db.NamedExecContext(ctx, query, history)
	if err != nil {
		r.logger.Error("Failed to create order status history", zap.Error(err), zap.String("order_id", history.OrderID.String()))
		return fmt.Errorf("failed to create order status history: %w", err)
	}

	r.logger.Info("Order status history created", zap.String("id", history.ID.String()), zap.String("order_id", history.OrderID.String()))
	return nil
}

// UpdateOrdersStatus updates the status of multiple orders
func (r *orderRepository) UpdateOrdersStatus(ctx context.Context, orderIDs []uuid.UUID, status OrderStatus) error {
	if len(orderIDs) == 0 {
		return nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(orderIDs))
	args := make([]interface{}, len(orderIDs)+1)

	for i, id := range orderIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	args[len(orderIDs)] = status

	query := fmt.Sprintf(`
		UPDATE orders 
		SET status = $%d, updated_at = NOW() 
		WHERE id IN (%s)`,
		len(orderIDs)+1,
		strings.Join(placeholders, ","))

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update orders status", zap.Error(err), zap.String("status", string(status)))
		return fmt.Errorf("failed to update orders status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	r.logger.Info("Orders status updated", zap.Int64("rows_affected", rowsAffected), zap.String("status", string(status)))
	return nil
}

// GetOrdersForSync retrieves orders that need to be synchronized with dYdX
func (r *orderRepository) GetOrdersForSync(ctx context.Context, limit int) ([]*Order, error) {
	query := `
		SELECT * FROM orders 
		WHERE status IN ('PENDING', 'OPEN', 'PARTIALLY_FILLED') 
		AND (placed_at IS NULL OR updated_at > placed_at + INTERVAL '5 minutes')
		ORDER BY created_at ASC`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	var orders []*Order
	err := r.db.SelectContext(ctx, &orders, query)
	if err != nil {
		r.logger.Error("Failed to get orders for sync", zap.Error(err))
		return nil, fmt.Errorf("failed to get orders for sync: %w", err)
	}

	return orders, nil
}
