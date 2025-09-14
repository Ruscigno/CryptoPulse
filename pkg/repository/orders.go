package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// OrderSide represents the side of an order
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderType represents the type of an order
type OrderType string

const (
	OrderTypeMarket           OrderType = "MARKET"
	OrderTypeLimit            OrderType = "LIMIT"
	OrderTypeStopMarket       OrderType = "STOP_MARKET"
	OrderTypeStopLimit        OrderType = "STOP_LIMIT"
	OrderTypeTakeProfitMarket OrderType = "TAKE_PROFIT_MARKET"
	OrderTypeTakeProfitLimit  OrderType = "TAKE_PROFIT_LIMIT"
)

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderStatusPending         OrderStatus = "PENDING"
	OrderStatusOpen            OrderStatus = "OPEN"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCancelled       OrderStatus = "CANCELLED"
	OrderStatusRejected        OrderStatus = "REJECTED"
	OrderStatusExpired         OrderStatus = "EXPIRED"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
)

// TimeInForce represents the time in force for an order
type TimeInForce string

const (
	TimeInForceGTT TimeInForce = "GTT" // Good Till Time
	TimeInForceFOK TimeInForce = "FOK" // Fill Or Kill
	TimeInForceIOC TimeInForce = "IOC" // Immediate Or Cancel
)

// Order represents an order in the database
type Order struct {
	ID       uuid.UUID `db:"id" json:"id"`
	ClientID string    `db:"client_id" json:"client_id"`
	OrderID  *string   `db:"order_id" json:"order_id,omitempty"`

	// Order details
	Market string    `db:"market" json:"market"`
	Side   OrderSide `db:"side" json:"side"`
	Type   OrderType `db:"type" json:"type"`
	Size   float64   `db:"size" json:"size"`
	Price  *float64  `db:"price" json:"price,omitempty"`

	// dYdX specific fields
	Quantums *int64 `db:"quantums" json:"quantums,omitempty"`
	Subticks *int64 `db:"subticks" json:"subticks,omitempty"`

	// Order execution parameters
	TimeInForce       TimeInForce `db:"time_in_force" json:"time_in_force"`
	GoodTilBlock      *int32      `db:"good_til_block" json:"good_til_block,omitempty"`
	GoodTilBlockTime  *time.Time  `db:"good_til_block_time" json:"good_til_block_time,omitempty"`

	// Order status and execution
	Status           OrderStatus `db:"status" json:"status"`
	FilledSize       float64     `db:"filled_size" json:"filled_size"`
	RemainingSize    float64     `db:"remaining_size" json:"remaining_size"`
	AverageFillPrice *float64    `db:"average_fill_price" json:"average_fill_price,omitempty"`

	// Transaction information
	TxHash      *string `db:"tx_hash" json:"tx_hash,omitempty"`
	BlockHeight *int64  `db:"block_height" json:"block_height,omitempty"`

	// Fees and costs
	MakerFee *float64 `db:"maker_fee" json:"maker_fee,omitempty"`
	TakerFee *float64 `db:"taker_fee" json:"taker_fee,omitempty"`
	GasUsed  *int64   `db:"gas_used" json:"gas_used,omitempty"`
	GasFee   *float64 `db:"gas_fee" json:"gas_fee,omitempty"`

	// Error handling
	ErrorMessage *string `db:"error_message" json:"error_message,omitempty"`
	RetryCount   int     `db:"retry_count" json:"retry_count"`

	// Timestamps
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
	PlacedAt    *time.Time `db:"placed_at" json:"placed_at,omitempty"`
	FilledAt    *time.Time `db:"filled_at" json:"filled_at,omitempty"`
	CancelledAt *time.Time `db:"cancelled_at" json:"cancelled_at,omitempty"`
}

// OrderStatusHistory represents a status change in the order history
type OrderStatusHistory struct {
	ID        uuid.UUID   `db:"id" json:"id"`
	OrderID   uuid.UUID   `db:"order_id" json:"order_id"`
	OldStatus *OrderStatus `db:"old_status" json:"old_status,omitempty"`
	NewStatus OrderStatus `db:"new_status" json:"new_status"`

	// Additional context
	FilledSize    *float64 `db:"filled_size" json:"filled_size,omitempty"`
	RemainingSize *float64 `db:"remaining_size" json:"remaining_size,omitempty"`
	FillPrice     *float64 `db:"fill_price" json:"fill_price,omitempty"`

	// Transaction details
	TxHash      *string `db:"tx_hash" json:"tx_hash,omitempty"`
	BlockHeight *int64  `db:"block_height" json:"block_height,omitempty"`

	// Reason for status change
	Reason *string `db:"reason" json:"reason,omitempty"`

	// Timestamp
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// OrderRepository defines the interface for order data access
type OrderRepository interface {
	// Order CRUD operations
	CreateOrder(ctx context.Context, order *Order) error
	GetOrderByID(ctx context.Context, id uuid.UUID) (*Order, error)
	GetOrderByClientID(ctx context.Context, clientID string) (*Order, error)
	GetOrderByOrderID(ctx context.Context, orderID string) (*Order, error)
	UpdateOrder(ctx context.Context, order *Order) error
	DeleteOrder(ctx context.Context, id uuid.UUID) error

	// Order queries
	ListOrders(ctx context.Context, filters OrderFilters) ([]*Order, error)
	ListActiveOrders(ctx context.Context, market string) ([]*Order, error)
	ListOrdersByStatus(ctx context.Context, status OrderStatus, limit int) ([]*Order, error)
	CountOrdersByStatus(ctx context.Context, status OrderStatus) (int, error)

	// Order status history
	GetOrderHistory(ctx context.Context, orderID uuid.UUID) ([]*OrderStatusHistory, error)
	CreateOrderStatusHistory(ctx context.Context, history *OrderStatusHistory) error

	// Batch operations
	UpdateOrdersStatus(ctx context.Context, orderIDs []uuid.UUID, status OrderStatus) error
	GetOrdersForSync(ctx context.Context, limit int) ([]*Order, error)
}

// OrderFilters represents filters for querying orders
type OrderFilters struct {
	Market    *string
	Side      *OrderSide
	Type      *OrderType
	Status    *OrderStatus
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Limit     int
	Offset    int
}

// orderRepository implements the OrderRepository interface
type orderRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewOrderRepository creates a new order repository
func NewOrderRepository(db *sqlx.DB, logger *zap.Logger) OrderRepository {
	return &orderRepository{
		db:     db,
		logger: logger,
	}
}

// CreateOrder creates a new order in the database
func (r *orderRepository) CreateOrder(ctx context.Context, order *Order) error {
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}
	
	query := `
		INSERT INTO orders (
			id, client_id, market, side, type, size, price, quantums, subticks,
			time_in_force, good_til_block, good_til_block_time, status,
			filled_size, remaining_size, retry_count
		) VALUES (
			:id, :client_id, :market, :side, :type, :size, :price, :quantums, :subticks,
			:time_in_force, :good_til_block, :good_til_block_time, :status,
			:filled_size, :remaining_size, :retry_count
		)`

	_, err := r.db.NamedExecContext(ctx, query, order)
	if err != nil {
		r.logger.Error("Failed to create order", zap.Error(err), zap.String("client_id", order.ClientID))
		return fmt.Errorf("failed to create order: %w", err)
	}

	r.logger.Info("Order created", zap.String("id", order.ID.String()), zap.String("client_id", order.ClientID))
	return nil
}

// GetOrderByID retrieves an order by its ID
func (r *orderRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (*Order, error) {
	var order Order
	query := `SELECT * FROM orders WHERE id = $1`
	
	err := r.db.GetContext(ctx, &order, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found: %s", id)
		}
		r.logger.Error("Failed to get order by ID", zap.Error(err), zap.String("id", id.String()))
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return &order, nil
}

// GetOrderByClientID retrieves an order by its client ID
func (r *orderRepository) GetOrderByClientID(ctx context.Context, clientID string) (*Order, error) {
	var order Order
	query := `SELECT * FROM orders WHERE client_id = $1`
	
	err := r.db.GetContext(ctx, &order, query, clientID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found: %s", clientID)
		}
		r.logger.Error("Failed to get order by client ID", zap.Error(err), zap.String("client_id", clientID))
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return &order, nil
}

// GetOrderByOrderID retrieves an order by its dYdX order ID
func (r *orderRepository) GetOrderByOrderID(ctx context.Context, orderID string) (*Order, error) {
	var order Order
	query := `SELECT * FROM orders WHERE order_id = $1`
	
	err := r.db.GetContext(ctx, &order, query, orderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found: %s", orderID)
		}
		r.logger.Error("Failed to get order by order ID", zap.Error(err), zap.String("order_id", orderID))
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return &order, nil
}

// UpdateOrder updates an existing order
func (r *orderRepository) UpdateOrder(ctx context.Context, order *Order) error {
	query := `
		UPDATE orders SET
			order_id = :order_id,
			status = :status,
			filled_size = :filled_size,
			remaining_size = :remaining_size,
			average_fill_price = :average_fill_price,
			tx_hash = :tx_hash,
			block_height = :block_height,
			maker_fee = :maker_fee,
			taker_fee = :taker_fee,
			gas_used = :gas_used,
			gas_fee = :gas_fee,
			error_message = :error_message,
			retry_count = :retry_count,
			placed_at = :placed_at,
			filled_at = :filled_at,
			cancelled_at = :cancelled_at,
			updated_at = NOW()
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, order)
	if err != nil {
		r.logger.Error("Failed to update order", zap.Error(err), zap.String("id", order.ID.String()))
		return fmt.Errorf("failed to update order: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("order not found: %s", order.ID)
	}

	r.logger.Info("Order updated", zap.String("id", order.ID.String()))
	return nil
}

// DeleteOrder deletes an order by ID
func (r *orderRepository) DeleteOrder(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM orders WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete order", zap.Error(err), zap.String("id", id.String()))
		return fmt.Errorf("failed to delete order: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("order not found: %s", id)
	}

	r.logger.Info("Order deleted", zap.String("id", id.String()))
	return nil
}
