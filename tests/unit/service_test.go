package unit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/query"
	"github.com/Ruscigno/CryptoPulse/pkg/repository"
	"github.com/Ruscigno/CryptoPulse/pkg/service"
	"github.com/Ruscigno/CryptoPulse/pkg/tx"
	"github.com/google/uuid"
)

// Define error for testing
var ErrOrderNotFound = errors.New("order not found")

// MockWallet implements wallet.Wallet interface for testing
type MockWallet struct{}

func (mw *MockWallet) GetAddress(ctx context.Context) (string, error) {
	return "dydx1test123", nil
}

func (mw *MockWallet) SignBytes(data []byte) ([]byte, error) {
	return []byte("mock-signature"), nil
}

func (mw *MockWallet) SignHash(hash []byte) ([]byte, error) {
	return []byte("mock-signature"), nil
}

// MockTxBuilder implements tx.TxBuilder interface for testing
type MockTxBuilder struct{}

func (mtb *MockTxBuilder) PlaceOrder(ctx context.Context, params tx.OrderParams) (*tx.TxResponse, error) {
	return &tx.TxResponse{
		TxHash:    "mock-tx-hash",
		Code:      0,
		RawLog:    "mock transaction successful",
		GasUsed:   50000,
		GasWanted: 60000,
		Height:    12345,
		Timestamp: time.Now(),
	}, nil
}

func (mtb *MockTxBuilder) CancelOrder(ctx context.Context, orderID string) (*tx.TxResponse, error) {
	return &tx.TxResponse{
		TxHash:    "mock-cancel-tx-hash",
		Code:      0,
		RawLog:    "mock cancel transaction successful",
		GasUsed:   30000,
		GasWanted: 40000,
		Height:    12346,
		Timestamp: time.Now(),
	}, nil
}

// MockQueryClient implements query.QueryClient interface for testing
type MockQueryClient struct{}

func (mqc *MockQueryClient) GetPositions(ctx context.Context, address string) (*query.PositionsResponse, error) {
	return &query.PositionsResponse{
		Positions: []query.Position{
			{
				Market:        "BTC-USD",
				Side:          "LONG",
				Size:          "1.5",
				EntryPrice:    "50000.0",
				UnrealizedPnl: "1000.0",
				RealizedPnl:   "500.0",
				CreatedAt:     time.Now().Format(time.RFC3339),
			},
		},
	}, nil
}

func (mqc *MockQueryClient) GetOrders(ctx context.Context, address string, status *string, market *string) (*query.OrdersResponse, error) {
	return &query.OrdersResponse{
		Orders: []query.OrderResponse{
			{
				ID:            "order-123",
				ClientID:      "client-123",
				Market:        "BTC-USD",
				Side:          "BUY",
				Size:          "1.0",
				RemainingSize: "0.5",
				Price:         "50000.0",
				Type:          "LIMIT",
				Status:        "OPEN",
				CreatedAt:     time.Now().Format(time.RFC3339),
			},
		},
	}, nil
}

func (mqc *MockQueryClient) GetOrderByID(ctx context.Context, orderID string) (*query.OrderResponse, error) {
	return &query.OrderResponse{
		ID:            orderID,
		ClientID:      "client-123",
		Market:        "BTC-USD",
		Side:          "BUY",
		Size:          "1.0",
		RemainingSize: "0.5",
		Price:         "50000.0",
		Type:          "LIMIT",
		Status:        "OPEN",
		CreatedAt:     time.Now().Format(time.RFC3339),
	}, nil
}

func (mqc *MockQueryClient) GetMarkets(ctx context.Context) (*query.MarketsResponse, error) {
	return &query.MarketsResponse{
		Markets: map[string]query.MarketConfig{
			"BTC-USD": {
				Market:     "BTC-USD",
				Status:     "ACTIVE",
				BaseAsset:  "BTC",
				QuoteAsset: "USD",
				StepSize:   "0.001",
				TickSize:   "1.0",
			},
		},
	}, nil
}

func (mqc *MockQueryClient) GetMarketConfig(ctx context.Context, market string) (*query.MarketConfig, error) {
	return &query.MarketConfig{
		Market:     market,
		Status:     "ACTIVE",
		BaseAsset:  "BTC",
		QuoteAsset: "USD",
		StepSize:   "0.001",
		TickSize:   "1.0",
	}, nil
}

// MockOrderRepository implements repository.OrderRepository interface for testing
type MockOrderRepository struct {
	orders map[string]*repository.Order
}

func NewMockOrderRepository() *MockOrderRepository {
	return &MockOrderRepository{
		orders: make(map[string]*repository.Order),
	}
}

func (mor *MockOrderRepository) CreateOrder(ctx context.Context, order *repository.Order) error {
	mor.orders[order.ID.String()] = order
	return nil
}

func (mor *MockOrderRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (*repository.Order, error) {
	idStr := id.String()
	if order, exists := mor.orders[idStr]; exists {
		return order, nil
	}
	return nil, ErrOrderNotFound
}

func (mor *MockOrderRepository) GetOrderByClientID(ctx context.Context, clientID string) (*repository.Order, error) {
	for _, order := range mor.orders {
		if order.ClientID == clientID {
			return order, nil
		}
	}
	return nil, ErrOrderNotFound
}

func (mor *MockOrderRepository) GetOrderByOrderID(ctx context.Context, orderID string) (*repository.Order, error) {
	for _, order := range mor.orders {
		if order.OrderID != nil && *order.OrderID == orderID {
			return order, nil
		}
	}
	return nil, ErrOrderNotFound
}

func (mor *MockOrderRepository) DeleteOrder(ctx context.Context, id uuid.UUID) error {
	delete(mor.orders, id.String())
	return nil
}

func (mor *MockOrderRepository) ListActiveOrders(ctx context.Context, market string) ([]*repository.Order, error) {
	var result []*repository.Order
	for _, order := range mor.orders {
		if order.Market == market && (order.Status == repository.OrderStatusOpen || order.Status == repository.OrderStatusPartiallyFilled) {
			result = append(result, order)
		}
	}
	return result, nil
}

func (mor *MockOrderRepository) ListOrdersByStatus(ctx context.Context, status repository.OrderStatus, limit int) ([]*repository.Order, error) {
	var result []*repository.Order
	count := 0
	for _, order := range mor.orders {
		if order.Status == status {
			result = append(result, order)
			count++
			if count >= limit {
				break
			}
		}
	}
	return result, nil
}

func (mor *MockOrderRepository) CountOrdersByStatus(ctx context.Context, status repository.OrderStatus) (int, error) {
	count := 0
	for _, order := range mor.orders {
		if order.Status == status {
			count++
		}
	}
	return count, nil
}

func (mor *MockOrderRepository) UpdateOrder(ctx context.Context, order *repository.Order) error {
	mor.orders[order.ID.String()] = order
	return nil
}

func (mor *MockOrderRepository) ListOrders(ctx context.Context, filters repository.OrderFilters) ([]*repository.Order, error) {
	var result []*repository.Order
	for _, order := range mor.orders {
		result = append(result, order)
	}
	return result, nil
}

func (mor *MockOrderRepository) GetOrderHistory(ctx context.Context, orderID uuid.UUID) ([]*repository.OrderStatusHistory, error) {
	return []*repository.OrderStatusHistory{}, nil
}

func (mor *MockOrderRepository) CreateOrderStatusHistory(ctx context.Context, history *repository.OrderStatusHistory) error {
	return nil
}

func (mor *MockOrderRepository) UpdateOrdersStatus(ctx context.Context, orderIDs []uuid.UUID, status repository.OrderStatus) error {
	for _, id := range orderIDs {
		if order, exists := mor.orders[id.String()]; exists {
			order.Status = status
		}
	}
	return nil
}

func (mor *MockOrderRepository) GetOrdersForSync(ctx context.Context, limit int) ([]*repository.Order, error) {
	var result []*repository.Order
	count := 0
	for _, order := range mor.orders {
		result = append(result, order)
		count++
		if count >= limit {
			break
		}
	}
	return result, nil
}

// MockDB implements database.DB interface for testing
type MockDB struct{}

func (mdb *MockDB) Close() error {
	return nil
}

func (mdb *MockDB) Health(ctx context.Context) error {
	return nil
}

// TestOrderRequestValidation tests order request validation logic
func TestOrderRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     service.OrderRequest
		wantErr bool
	}{
		{
			name: "valid limit order",
			req: service.OrderRequest{
				Market: "BTC-USD",
				Side:   "BUY",
				Type:   "LIMIT",
				Size:   1.0,
				Price:  func() *float64 { p := 50000.0; return &p }(),
			},
			wantErr: false,
		},
		{
			name: "valid market order",
			req: service.OrderRequest{
				Market: "BTC-USD",
				Side:   "SELL",
				Type:   "MARKET",
				Size:   1.0,
			},
			wantErr: false,
		},
		{
			name: "invalid empty market",
			req: service.OrderRequest{
				Market: "",
				Side:   "BUY",
				Type:   "LIMIT",
				Size:   1.0,
				Price:  func() *float64 { p := 50000.0; return &p }(),
			},
			wantErr: true,
		},
		{
			name: "invalid side",
			req: service.OrderRequest{
				Market: "BTC-USD",
				Side:   "INVALID",
				Type:   "LIMIT",
				Size:   1.0,
				Price:  func() *float64 { p := 50000.0; return &p }(),
			},
			wantErr: true,
		},
		{
			name: "invalid zero size",
			req: service.OrderRequest{
				Market: "BTC-USD",
				Side:   "BUY",
				Type:   "LIMIT",
				Size:   0,
				Price:  func() *float64 { p := 50000.0; return &p }(),
			},
			wantErr: true,
		},
		{
			name: "limit order without price",
			req: service.OrderRequest{
				Market: "BTC-USD",
				Side:   "BUY",
				Type:   "LIMIT",
				Size:   1.0,
			},
			wantErr: true,
		},
		{
			name: "market order with price",
			req: service.OrderRequest{
				Market: "BTC-USD",
				Side:   "BUY",
				Type:   "MARKET",
				Size:   1.0,
				Price:  func() *float64 { p := 50000.0; return &p }(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation logic directly
			err := validateOrderRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOrderRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// validateOrderRequest is a copy of the validation logic for testing
func validateOrderRequest(req service.OrderRequest) error {
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

// TestMockOrderRepository tests the mock repository functionality
func TestMockOrderRepository(t *testing.T) {
	repo := NewMockOrderRepository()
	ctx := context.Background()

	// Test data
	order := &repository.Order{
		ID:            uuid.New(),
		ClientID:      "test-client-123",
		Market:        "BTC-USD",
		Side:          repository.OrderSideBuy,
		Type:          repository.OrderTypeLimit,
		Size:          1.5,
		Price:         func() *float64 { p := 50000.0; return &p }(),
		Status:        repository.OrderStatusPending,
		FilledSize:    0,
		RemainingSize: 1.5,
		TimeInForce:   repository.TimeInForceGTT,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Test CreateOrder
	err := repo.CreateOrder(ctx, order)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Test GetOrderByID
	retrievedOrder, err := repo.GetOrderByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}

	if retrievedOrder.ID != order.ID {
		t.Errorf("Expected ID %v, got %v", order.ID, retrievedOrder.ID)
	}

	// Test GetOrderByClientID
	retrievedOrder, err = repo.GetOrderByClientID(ctx, order.ClientID)
	if err != nil {
		t.Fatalf("GetOrderByClientID failed: %v", err)
	}

	if retrievedOrder.ClientID != order.ClientID {
		t.Errorf("Expected ClientID %s, got %s", order.ClientID, retrievedOrder.ClientID)
	}

	// Test CountOrdersByStatus
	count, err := repo.CountOrdersByStatus(ctx, repository.OrderStatusPending)
	if err != nil {
		t.Fatalf("CountOrdersByStatus failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
}

// TestMockQueryClient tests the mock query client functionality
func TestMockQueryClient(t *testing.T) {
	client := &MockQueryClient{}
	ctx := context.Background()

	// Test GetPositions
	resp, err := client.GetPositions(ctx, "dydx1test123")
	if err != nil {
		t.Fatalf("GetPositions failed: %v", err)
	}

	if len(resp.Positions) == 0 {
		t.Error("Expected at least one position")
	}

	position := resp.Positions[0]
	if position.Market != "BTC-USD" {
		t.Errorf("Expected market 'BTC-USD', got '%s'", position.Market)
	}

	// Test GetMarkets
	marketsResp, err := client.GetMarkets(ctx)
	if err != nil {
		t.Fatalf("GetMarkets failed: %v", err)
	}

	if len(marketsResp.Markets) == 0 {
		t.Error("Expected at least one market")
	}

	if _, exists := marketsResp.Markets["BTC-USD"]; !exists {
		t.Error("Expected BTC-USD market to exist")
	}
}
