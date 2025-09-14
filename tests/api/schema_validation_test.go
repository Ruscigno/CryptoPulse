package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/service"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchemaValidation validates that Go structs match OpenAPI schema definitions
func TestSchemaValidation(t *testing.T) {
	// Load OpenAPI specification
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("../../docs/api/openapi.yaml")
	require.NoError(t, err)

	// Validate OpenAPI document
	err = doc.Validate(context.Background())
	require.NoError(t, err)

	t.Run("OrderRequest", func(t *testing.T) {
		testOrderRequestSchema(t, doc)
	})

	t.Run("OrderResponse", func(t *testing.T) {
		testOrderResponseSchema(t, doc)
	})

	t.Run("CancelOrderRequest", func(t *testing.T) {
		testCancelOrderRequestSchema(t, doc)
	})

	t.Run("CancelOrderResponse", func(t *testing.T) {
		testCancelOrderResponseSchema(t, doc)
	})

	t.Run("PositionsResponse", func(t *testing.T) {
		testPositionsResponseSchema(t, doc)
	})

	t.Run("ClosePositionRequest", func(t *testing.T) {
		testClosePositionRequestSchema(t, doc)
	})

	t.Run("ClosePositionResponse", func(t *testing.T) {
		testClosePositionResponseSchema(t, doc)
	})

	t.Run("OrderStatusResponse", func(t *testing.T) {
		testOrderStatusResponseSchema(t, doc)
	})

	t.Run("OrderHistoryResponse", func(t *testing.T) {
		testOrderHistoryResponseSchema(t, doc)
	})

	t.Run("HealthResponse", func(t *testing.T) {
		testHealthResponseSchema(t, doc)
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		testErrorResponseSchema(t, doc)
	})
}

func testOrderRequestSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["OrderRequest"]
	require.NotNil(t, schema, "OrderRequest schema should exist in OpenAPI spec")

	// Create sample OrderRequest
	req := service.OrderRequest{
		Market:       "BTC-USD",
		Side:         "BUY",
		Type:         "MARKET",
		Size:         0.001,
		Price:        floatPtr(50000.0),
		TimeInForce:  "GTT",
		GoodTilBlock: uint32Ptr(12345),
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "OrderRequest should match OpenAPI schema")

	// Validate required fields exist in schema
	properties := schema.Value.Properties
	assert.Contains(t, properties, "market", "Schema should have market field")
	assert.Contains(t, properties, "side", "Schema should have side field")
	assert.Contains(t, properties, "type", "Schema should have type field")
	assert.Contains(t, properties, "size", "Schema should have size field")

	// Validate required fields are marked as required
	required := schema.Value.Required
	assert.Contains(t, required, "market", "market should be required")
	assert.Contains(t, required, "side", "side should be required")
	assert.Contains(t, required, "type", "type should be required")
	assert.Contains(t, required, "size", "size should be required")
}

func testOrderResponseSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["OrderResponse"]
	require.NotNil(t, schema, "OrderResponse schema should exist in OpenAPI spec")

	// Create sample OrderResponse
	resp := service.OrderResponse{
		OrderID:   "test-order-123",
		ClientID:  "client-123",
		Status:    "pending",
		TxHash:    "0x123abc",
		Message:   "Order placed successfully",
		CreatedAt: "2024-01-15T10:30:00Z",
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "OrderResponse should match OpenAPI schema")

	// Validate required fields
	properties := schema.Value.Properties
	assert.Contains(t, properties, "orderId", "Schema should have orderId field")
	assert.Contains(t, properties, "status", "Schema should have status field")
	assert.Contains(t, properties, "createdAt", "Schema should have createdAt field")
}

func testCancelOrderRequestSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["CancelOrderRequest"]
	require.NotNil(t, schema, "CancelOrderRequest schema should exist in OpenAPI spec")

	// Create sample CancelOrderRequest
	req := service.CancelOrderRequest{
		OrderID:  "test-order-123",
		ClientID: "client-123",
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "CancelOrderRequest should match OpenAPI schema")

	// Validate fields exist in schema
	properties := schema.Value.Properties
	assert.Contains(t, properties, "orderId", "Schema should have orderId field")
	assert.Contains(t, properties, "clientId", "Schema should have clientId field")
}

func testCancelOrderResponseSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["CancelOrderResponse"]
	require.NotNil(t, schema, "CancelOrderResponse schema should exist in OpenAPI spec")

	// Create sample CancelOrderResponse
	resp := service.CancelOrderResponse{
		OrderID:   "test-order-123",
		Status:    "cancelled",
		TxHash:    "0x456def",
		Message:   "Order cancelled successfully",
		UpdatedAt: "2024-01-15T10:35:00Z",
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "CancelOrderResponse should match OpenAPI schema")
}

func testPositionsResponseSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["PositionsResponse"]
	require.NotNil(t, schema, "PositionsResponse schema should exist in OpenAPI spec")

	// Create sample PositionsResponse
	resp := service.PositionsResponse{
		Positions: []service.Position{
			{
				Market:        "BTC-USD",
				Side:          "LONG",
				Size:          "1.5",
				EntryPrice:    "50000.0",
				UnrealizedPnl: "1500.0",
				RealizedPnl:   "0.0",
				CreatedAt:     "2024-01-15T09:00:00Z",
			},
		},
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "PositionsResponse should match OpenAPI schema")

	// Validate positions array structure
	properties := schema.Value.Properties
	assert.Contains(t, properties, "positions", "Schema should have positions field")
}

func testClosePositionRequestSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["ClosePositionRequest"]
	require.NotNil(t, schema, "ClosePositionRequest schema should exist in OpenAPI spec")

	// Create sample ClosePositionRequest
	req := service.ClosePositionRequest{
		Market: "BTC-USD",
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "ClosePositionRequest should match OpenAPI schema")

	// Validate required fields
	required := schema.Value.Required
	assert.Contains(t, required, "market", "market should be required")
}

func testClosePositionResponseSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["ClosePositionResponse"]
	require.NotNil(t, schema, "ClosePositionResponse schema should exist in OpenAPI spec")

	// Create sample ClosePositionResponse
	resp := service.ClosePositionResponse{
		OrderID:   "close-order-123",
		Market:    "BTC-USD",
		Status:    "pending",
		TxHash:    "0x789ghi",
		Message:   "Position close order placed successfully",
		CreatedAt: "2024-01-15T10:40:00Z",
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "ClosePositionResponse should match OpenAPI schema")
}

func testOrderStatusResponseSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["OrderStatusResponse"]
	require.NotNil(t, schema, "OrderStatusResponse schema should exist in OpenAPI spec")

	// Create sample OrderStatusResponse
	resp := service.OrderStatusResponse{
		OrderID:       "test-order-123",
		ClientID:      "client-123",
		Status:        "FILLED",
		Market:        "BTC-USD",
		Side:          "BUY",
		Size:          "1.0",
		FilledSize:    "1.0",
		RemainingSize: "0.0",
		Price:         stringPtr("50000.0"),
		CreatedAt:     "2024-01-15T10:30:00Z",
		UpdatedAt:     "2024-01-15T10:32:00Z",
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "OrderStatusResponse should match OpenAPI schema")

	// Validate required fields
	properties := schema.Value.Properties
	assert.Contains(t, properties, "orderId", "Schema should have orderId field")
	assert.Contains(t, properties, "status", "Schema should have status field")
	assert.Contains(t, properties, "market", "Schema should have market field")
	assert.Contains(t, properties, "side", "Schema should have side field")
}

func testOrderHistoryResponseSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["OrderHistoryResponse"]
	require.NotNil(t, schema, "OrderHistoryResponse schema should exist in OpenAPI spec")

	// Create sample OrderHistoryResponse
	resp := service.OrderHistoryResponse{
		Orders: []service.OrderStatusResponse{
			{
				OrderID:       "order-1",
				ClientID:      "client-1",
				Status:        "FILLED",
				Market:        "BTC-USD",
				Side:          "BUY",
				Size:          "1.0",
				FilledSize:    "1.0",
				RemainingSize: "0.0",
				CreatedAt:     "2024-01-15T10:30:00Z",
				UpdatedAt:     "2024-01-15T10:32:00Z",
			},
		},
		Total: 1,
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "OrderHistoryResponse should match OpenAPI schema")

	// Validate required fields
	properties := schema.Value.Properties
	assert.Contains(t, properties, "orders", "Schema should have orders field")
	assert.Contains(t, properties, "total", "Schema should have total field")
}

func testHealthResponseSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["HealthResponse"]
	require.NotNil(t, schema, "HealthResponse schema should exist in OpenAPI spec")

	// Create sample HealthResponse
	resp := service.HealthResponse{
		Status:    service.HealthStatus("healthy"),
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Components: []service.ComponentHealth{
			{
				Name:      "database",
				Status:    service.HealthStatus("healthy"),
				Message:   "Database is healthy",
				Timestamp: time.Now(),
				Duration:  "2ms",
			},
		},
		Uptime: "1h30m",
	}

	// Convert to JSON and validate against schema
	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var data interface{}
	err = json.Unmarshal(jsonData, &data)
	require.NoError(t, err)

	err = schema.Value.VisitJSON(data)
	assert.NoError(t, err, "HealthResponse should match OpenAPI schema")

	// Validate required fields
	properties := schema.Value.Properties
	assert.Contains(t, properties, "status", "Schema should have status field")
	assert.Contains(t, properties, "timestamp", "Schema should have timestamp field")
	assert.Contains(t, properties, "version", "Schema should have version field")
	assert.Contains(t, properties, "components", "Schema should have components field")
}

func testErrorResponseSchema(t *testing.T, doc *openapi3.T) {
	schema := doc.Components.Schemas["ErrorResponse"]
	require.NotNil(t, schema, "ErrorResponse schema should exist in OpenAPI spec")

	// Create sample ErrorResponse (based on pkg/errors/errors.go)
	errorResp := map[string]interface{}{
		"error":     "error",
		"code":      "VALIDATION_ERROR",
		"message":   "Invalid order request",
		"details":   "Market field is required",
		"timestamp": "2024-01-15T10:45:00Z",
		"requestId": "req-123e4567-e89b-12d3",
		"metadata": map[string]interface{}{
			"field": "market",
		},
	}

	// Validate against schema
	err := schema.Value.VisitJSON(errorResp)
	assert.NoError(t, err, "ErrorResponse should match OpenAPI schema")

	// Validate required fields
	properties := schema.Value.Properties
	assert.Contains(t, properties, "error", "Schema should have error field")
	assert.Contains(t, properties, "code", "Schema should have code field")
	assert.Contains(t, properties, "message", "Schema should have message field")
	assert.Contains(t, properties, "timestamp", "Schema should have timestamp field")
}

func floatPtr(f float64) *float64 {
	return &f
}

func uint32Ptr(u uint32) *uint32 {
	return &u
}

func stringPtr(s string) *string {
	return &s
}
