package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/config"
	"github.com/Ruscigno/CryptoPulse/pkg/database"
	"github.com/Ruscigno/CryptoPulse/pkg/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) (*database.DB, func()) {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set INTEGRATION_TESTS=true to run")
	}

	logger := zap.NewNop()
	cfg := config.Config{
		DatabaseURL: getTestDatabaseURL(),
	}

	db, err := database.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

// getTestDatabaseURL returns the test database URL
func getTestDatabaseURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://postgres:password@localhost:5432/cryptopulse_test?sslmode=disable"
}

// TestDatabaseConnection tests basic database connectivity
func TestDatabaseConnection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.Health(ctx)
	if err != nil {
		t.Fatalf("Database health check failed: %v", err)
	}
}

// TestOrderRepository tests the order repository with real database
func TestOrderRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	repo := repository.NewOrderRepository(db.DB, logger)

	ctx := context.Background()

	// Test data
	order := &repository.Order{
		ID:            uuid.New(),
		ClientID:      "test-client-" + uuid.New().String(),
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

	if retrievedOrder.Market != order.Market {
		t.Errorf("Expected market %s, got %s", order.Market, retrievedOrder.Market)
	}

	// Test GetOrderByClientID
	retrievedOrder, err = repo.GetOrderByClientID(ctx, order.ClientID)
	if err != nil {
		t.Fatalf("GetOrderByClientID failed: %v", err)
	}

	if retrievedOrder.ClientID != order.ClientID {
		t.Errorf("Expected ClientID %s, got %s", order.ClientID, retrievedOrder.ClientID)
	}

	// Test UpdateOrder
	order.Status = repository.OrderStatusOpen
	order.FilledSize = 0.5
	order.RemainingSize = 1.0
	order.UpdatedAt = time.Now()

	err = repo.UpdateOrder(ctx, order)
	if err != nil {
		t.Fatalf("UpdateOrder failed: %v", err)
	}

	// Verify update
	retrievedOrder, err = repo.GetOrderByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID after update failed: %v", err)
	}

	if retrievedOrder.Status != repository.OrderStatusOpen {
		t.Errorf("Expected status %s, got %s", repository.OrderStatusOpen, retrievedOrder.Status)
	}

	if retrievedOrder.FilledSize != 0.5 {
		t.Errorf("Expected FilledSize 0.5, got %f", retrievedOrder.FilledSize)
	}

	// Test ListOrders
	filters := repository.OrderFilters{
		Market: &order.Market,
		Limit:  10,
		Offset: 0,
	}

	orders, err := repo.ListOrders(ctx, filters)
	if err != nil {
		t.Fatalf("ListOrders failed: %v", err)
	}

	if len(orders) == 0 {
		t.Error("Expected at least one order")
	}

	// Find our order in the list
	found := false
	for _, o := range orders {
		if o.ID == order.ID {
			found = true
			break
		}
	}

	if !found {
		t.Error("Created order not found in list")
	}
}

// TestOrderStatusHistory tests order status history functionality
func TestOrderStatusHistory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	repo := repository.NewOrderRepository(db.DB, logger)

	ctx := context.Background()

	// Create an order
	order := &repository.Order{
		ID:            uuid.New(),
		ClientID:      "test-client-" + uuid.New().String(),
		Market:        "ETH-USD",
		Side:          repository.OrderSideSell,
		Type:          repository.OrderTypeMarket,
		Size:          2.0,
		Status:        repository.OrderStatusPending,
		FilledSize:    0,
		RemainingSize: 2.0,
		TimeInForce:   repository.TimeInForceIOC,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err := repo.CreateOrder(ctx, order)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Update order status multiple times to create history
	statuses := []repository.OrderStatus{
		repository.OrderStatusOpen,
		repository.OrderStatusPartiallyFilled,
		repository.OrderStatusFilled,
	}

	for _, status := range statuses {
		order.Status = status
		order.UpdatedAt = time.Now()

		err = repo.UpdateOrder(ctx, order)
		if err != nil {
			t.Fatalf("UpdateOrder failed: %v", err)
		}

		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Get order history
	history, err := repo.GetOrderHistory(ctx, order.ID)
	if err != nil {
		t.Fatalf("GetOrderHistory failed: %v", err)
	}

	// Should have at least the status changes we made
	if len(history) < len(statuses) {
		t.Errorf("Expected at least %d history entries, got %d", len(statuses), len(history))
	}

	// Verify the history entries
	for i, entry := range history {
		if i < len(statuses) {
			expectedStatus := statuses[i]
			if entry.NewStatus != expectedStatus {
				t.Errorf("Expected status %s at index %d, got %s", expectedStatus, i, entry.NewStatus)
			}
		}
	}
}

// TestDatabaseTransactions tests database transaction handling
func TestDatabaseTransactions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test transaction rollback
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert test data
	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders (id, client_id, market, side, type, size, price, status, filled_size, remaining_size, time_in_force, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, uuid.New(), "rollback-test", "BTC-USD", "BUY", "LIMIT", 1.0, 50000.0, "PENDING", 0.0, 1.0, "GTT", time.Now(), time.Now())

	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Rollback transaction
	err = tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to rollback transaction: %v", err)
	}

	// Verify data was not persisted
	var count int
	err = db.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE client_id = $1", "rollback-test").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query orders: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 orders after rollback, got %d", count)
	}
}

// TestDatabaseConnectionPool tests database connection pooling
func TestDatabaseConnectionPool(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	stats := db.GetStats()

	// Verify connection pool is configured
	if stats.MaxOpenConnections == 0 {
		t.Error("Expected MaxOpenConnections to be configured")
	}

	// Test concurrent connections
	ctx := context.Background()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			err := db.Health(ctx)
			if err != nil {
				t.Errorf("Health check failed in goroutine: %v", err)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check final stats
	finalStats := db.GetStats()
	if finalStats.OpenConnections < 0 {
		t.Error("Expected non-negative open connections")
	}
}
