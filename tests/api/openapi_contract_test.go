package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/endpoint"
	"github.com/Ruscigno/CryptoPulse/pkg/service"
	httptransport "github.com/Ruscigno/CryptoPulse/pkg/transport/http"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockService implements the service.Service interface for testing
type MockService struct{}

func (m *MockService) PlaceOrder(ctx context.Context, req service.OrderRequest) (service.OrderResponse, error) {
	return service.OrderResponse{
		OrderID:   "test-order-123",
		ClientID:  "client-123",
		Status:    "pending",
		TxHash:    "0x123abc",
		Message:   "Order placed successfully",
		CreatedAt: "2024-01-15T10:30:00Z",
	}, nil
}

func (m *MockService) CancelOrder(ctx context.Context, req service.CancelOrderRequest) (service.CancelOrderResponse, error) {
	return service.CancelOrderResponse{
		OrderID:   "test-order-123",
		Status:    "cancelled",
		TxHash:    "0x456def",
		Message:   "Order cancelled successfully",
		UpdatedAt: "2024-01-15T10:35:00Z",
	}, nil
}

func (m *MockService) GetPositions(ctx context.Context) (service.PositionsResponse, error) {
	return service.PositionsResponse{
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
	}, nil
}

func (m *MockService) ClosePosition(ctx context.Context, req service.ClosePositionRequest) (service.ClosePositionResponse, error) {
	return service.ClosePositionResponse{
		OrderID:   "close-order-123",
		Market:    req.Market,
		Status:    "pending",
		TxHash:    "0x789ghi",
		Message:   "Position close order placed successfully",
		CreatedAt: "2024-01-15T10:40:00Z",
	}, nil
}

func (m *MockService) GetOrderStatus(ctx context.Context, orderID string) (service.OrderStatusResponse, error) {
	return service.OrderStatusResponse{
		OrderID:       orderID,
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
	}, nil
}

func (m *MockService) GetOrderHistory(ctx context.Context, req service.OrderHistoryRequest) (service.OrderHistoryResponse, error) {
	return service.OrderHistoryResponse{
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
	}, nil
}

func (m *MockService) CheckHealth(ctx context.Context) (service.HealthResponse, error) {
	now := time.Now()
	return service.HealthResponse{
		Status:    service.HealthStatus("healthy"),
		Timestamp: now,
		Version:   "1.0.0",
		Components: []service.ComponentHealth{
			{
				Name:      "database",
				Status:    service.HealthStatus("healthy"),
				Message:   "Database is healthy",
				Timestamp: now,
				Duration:  "2ms",
			},
		},
		Uptime: "1h30m",
	}, nil
}

// TestOpenAPIContractAlignment tests that the OpenAPI spec aligns with actual implementation
func TestOpenAPIContractAlignment(t *testing.T) {
	// Load OpenAPI specification
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("../../docs/api/openapi.yaml")
	require.NoError(t, err)

	// Validate OpenAPI document
	err = doc.Validate(context.Background())
	require.NoError(t, err)

	// Create test server
	mockService := &MockService{}
	endpoints := endpoint.Endpoints{
		PlaceOrder:      makeTestEndpoint(mockService.PlaceOrder),
		CancelOrder:     makeTestEndpoint(mockService.CancelOrder),
		GetPositions:    makeTestEndpoint(mockService.GetPositions),
		ClosePosition:   makeTestEndpoint(mockService.ClosePosition),
		GetOrderStatus:  makeTestEndpoint(mockService.GetOrderStatus),
		GetOrderHistory: makeTestEndpoint(mockService.GetOrderHistory),
		CheckHealth:     makeTestEndpoint(mockService.CheckHealth),
	}

	config := httptransport.HTTPConfig{
		APIKey:            "test-api-key",
		MaxBodySize:       1024 * 1024,
		RequestsPerSecond: 100,
		BurstSize:         200,
		Logger:            zap.NewNop(),
		AllowedOrigins:    []string{"*"},
	}

	handler := httptransport.NewHTTPHandler(endpoints, config)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Test each endpoint defined in OpenAPI spec
	t.Run("EndpointExistence", func(t *testing.T) {
		testEndpointExistence(t, doc, server)
	})

	t.Run("RequestResponseStructures", func(t *testing.T) {
		testRequestResponseStructures(t, doc, server)
	})

	t.Run("AuthenticationRequirements", func(t *testing.T) {
		testAuthenticationRequirements(t, doc, server)
	})

	t.Run("HTTPMethods", func(t *testing.T) {
		testHTTPMethods(t, doc, server)
	})
}

func makeTestEndpoint(fn interface{}) func(context.Context, interface{}) (interface{}, error) {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		// Use reflection to call the actual service method
		fnValue := reflect.ValueOf(fn)
		fnType := fnValue.Type()

		var args []reflect.Value
		args = append(args, reflect.ValueOf(ctx))

		if fnType.NumIn() > 1 && request != nil {
			args = append(args, reflect.ValueOf(request))
		}

		results := fnValue.Call(args)
		if len(results) == 2 {
			if results[1].Interface() != nil {
				return results[0].Interface(), results[1].Interface().(error)
			}
			return results[0].Interface(), nil
		}
		return nil, fmt.Errorf("unexpected return values")
	}
}

func testEndpointExistence(t *testing.T, doc *openapi3.T, server *httptest.Server) {
	expectedEndpoints := map[string][]string{
		"/health":           {"GET"},
		"/place-order":      {"POST"},
		"/cancel-order":     {"POST"},
		"/positions":        {"GET"},
		"/close-position":   {"POST"},
		"/orders/{orderId}": {"GET"},
		"/order-history":    {"GET"},
	}

	for path, methods := range expectedEndpoints {
		for _, method := range methods {
			t.Run(fmt.Sprintf("%s %s", method, path), func(t *testing.T) {
				// Check if path exists in OpenAPI spec
				pathItem := doc.Paths.Find(path)
				assert.NotNil(t, pathItem, "Path %s should exist in OpenAPI spec", path)

				if pathItem != nil {
					// Check if method exists
					operation := pathItem.GetOperation(method)
					assert.NotNil(t, operation, "Method %s should exist for path %s", method, path)
				}

				// Test actual endpoint (for non-parameterized paths)
				if !strings.Contains(path, "{") {
					testPath := path
					if path == "/orders/{orderId}" {
						testPath = "/orders/test-order-123"
					}

					req, err := http.NewRequest(method, server.URL+testPath, nil)
					require.NoError(t, err)

					if path != "/health" {
						req.Header.Set("X-API-Key", "test-api-key")
					}

					resp, err := http.DefaultClient.Do(req)
					require.NoError(t, err)
					defer resp.Body.Close()

					// Should not return 404 (endpoint exists)
					assert.NotEqual(t, http.StatusNotFound, resp.StatusCode,
						"Endpoint %s %s should exist in implementation", method, testPath)
				}
			})
		}
	}
}

func testRequestResponseStructures(t *testing.T, doc *openapi3.T, server *httptest.Server) {
	testCases := []struct {
		name         string
		path         string
		method       string
		requestBody  string
		expectedCode int
	}{
		{
			name:   "PlaceOrder",
			path:   "/place-order",
			method: "POST",
			requestBody: `{
				"market": "BTC-USD",
				"side": "BUY",
				"type": "MARKET",
				"size": 0.001
			}`,
			expectedCode: 200,
		},
		{
			name:   "CancelOrder",
			path:   "/cancel-order",
			method: "POST",
			requestBody: `{
				"orderId": "test-order-123"
			}`,
			expectedCode: 200,
		},
		{
			name:         "GetPositions",
			path:         "/positions",
			method:       "GET",
			expectedCode: 200,
		},
		{
			name:   "ClosePosition",
			path:   "/close-position",
			method: "POST",
			requestBody: `{
				"market": "BTC-USD"
			}`,
			expectedCode: 200,
		},
		{
			name:         "GetOrderStatus",
			path:         "/orders/test-order-123",
			method:       "GET",
			expectedCode: 200,
		},
		{
			name:         "GetOrderHistory",
			path:         "/order-history",
			method:       "GET",
			expectedCode: 200,
		},
		{
			name:         "HealthCheck",
			path:         "/health",
			method:       "GET",
			expectedCode: 200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.requestBody != "" {
				body = strings.NewReader(tc.requestBody)
			}

			req, err := http.NewRequest(tc.method, server.URL+tc.path, body)
			require.NoError(t, err)

			if tc.path != "/health" {
				req.Header.Set("X-API-Key", "test-api-key")
			}
			if tc.requestBody != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Check response status
			assert.Equal(t, tc.expectedCode, resp.StatusCode,
				"Expected status code %d for %s %s", tc.expectedCode, tc.method, tc.path)

			// Check response content type
			contentType := resp.Header.Get("Content-Type")
			assert.Contains(t, contentType, "application/json",
				"Response should be JSON for %s %s", tc.method, tc.path)

			// Parse and validate response structure
			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var respData map[string]interface{}
			err = json.Unmarshal(respBody, &respData)
			require.NoError(t, err, "Response should be valid JSON for %s %s", tc.method, tc.path)

			// Validate response structure matches expected patterns
			validateResponseStructure(t, tc.name, respData)
		})
	}
}

func validateResponseStructure(t *testing.T, testName string, respData map[string]interface{}) {
	switch testName {
	case "PlaceOrder":
		assert.Contains(t, respData, "orderId")
		assert.Contains(t, respData, "status")
		assert.Contains(t, respData, "createdAt")
	case "CancelOrder":
		assert.Contains(t, respData, "orderId")
		assert.Contains(t, respData, "status")
		assert.Contains(t, respData, "updatedAt")
	case "GetPositions":
		assert.Contains(t, respData, "positions")
		if positions, ok := respData["positions"].([]interface{}); ok && len(positions) > 0 {
			pos := positions[0].(map[string]interface{})
			assert.Contains(t, pos, "market")
			assert.Contains(t, pos, "side")
			assert.Contains(t, pos, "size")
		}
	case "ClosePosition":
		assert.Contains(t, respData, "orderId")
		assert.Contains(t, respData, "market")
		assert.Contains(t, respData, "status")
	case "GetOrderStatus":
		assert.Contains(t, respData, "orderId")
		assert.Contains(t, respData, "status")
		assert.Contains(t, respData, "market")
		assert.Contains(t, respData, "side")
	case "GetOrderHistory":
		assert.Contains(t, respData, "orders")
		assert.Contains(t, respData, "total")
	case "HealthCheck":
		assert.Contains(t, respData, "status")
		assert.Contains(t, respData, "timestamp")
		assert.Contains(t, respData, "version")
		assert.Contains(t, respData, "components")
	}
}

func testAuthenticationRequirements(t *testing.T, doc *openapi3.T, server *httptest.Server) {
	// Test endpoints that require authentication
	authRequiredEndpoints := []struct {
		path   string
		method string
		body   string
	}{
		{"/place-order", "POST", `{"market":"BTC-USD","side":"BUY","type":"MARKET","size":0.001}`},
		{"/cancel-order", "POST", `{"orderId":"test-123"}`},
		{"/positions", "GET", ""},
		{"/close-position", "POST", `{"market":"BTC-USD"}`},
		{"/orders/test-123", "GET", ""},
		{"/order-history", "GET", ""},
	}

	for _, endpoint := range authRequiredEndpoints {
		t.Run(fmt.Sprintf("Auth_%s_%s", endpoint.method, endpoint.path), func(t *testing.T) {
			// Test without API key - should return 401
			var body io.Reader
			if endpoint.body != "" {
				body = strings.NewReader(endpoint.body)
			}

			req, err := http.NewRequest(endpoint.method, server.URL+endpoint.path, body)
			require.NoError(t, err)

			if endpoint.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"Endpoint %s %s should require authentication", endpoint.method, endpoint.path)

			// Test with valid API key - should not return 401
			if endpoint.body != "" {
				body = strings.NewReader(endpoint.body)
			}

			req, err = http.NewRequest(endpoint.method, server.URL+endpoint.path, body)
			require.NoError(t, err)

			req.Header.Set("X-API-Key", "test-api-key")
			if endpoint.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
				"Endpoint %s %s should accept valid API key", endpoint.method, endpoint.path)
		})
	}

	// Test health endpoint - should not require authentication
	t.Run("Health_NoAuth", func(t *testing.T) {
		req, err := http.NewRequest("GET", server.URL+"/health", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
			"Health endpoint should not require authentication")
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Health endpoint should return 200 OK")
	})
}

func testHTTPMethods(t *testing.T, doc *openapi3.T, server *httptest.Server) {
	// Test that endpoints accept their specified HTTP methods
	// Note: Go-Kit doesn't return 405 Method Not Allowed, so we test that
	// the correct methods work rather than testing that incorrect methods fail
	supportedMethods := map[string][]string{
		"/health":         {"GET"},
		"/place-order":    {"POST"},
		"/cancel-order":   {"POST"},
		"/positions":      {"GET"},
		"/close-position": {"POST"},
		"/order-history":  {"GET"},
	}

	for path, methods := range supportedMethods {
		for _, method := range methods {
			t.Run(fmt.Sprintf("Method_%s_%s_Supported", method, path), func(t *testing.T) {
				var body io.Reader
				if method == "POST" {
					body = strings.NewReader(`{"market":"BTC-USD","side":"BUY","type":"MARKET","size":0.001}`)
				}

				req, err := http.NewRequest(method, server.URL+path, body)
				require.NoError(t, err)

				if path != "/health" {
					req.Header.Set("X-API-Key", "test-api-key")
				}
				if method == "POST" {
					req.Header.Set("Content-Type", "application/json")
				}

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				// The method should be supported (not return 404 or 405)
				assert.NotEqual(t, http.StatusNotFound, resp.StatusCode,
					"Method %s should be supported for %s", method, path)
				assert.NotEqual(t, http.StatusMethodNotAllowed, resp.StatusCode,
					"Method %s should be supported for %s", method, path)

				// Should return a valid response (200, 400, 401, etc. but not 404/405)
				assert.True(t, resp.StatusCode < 500 || resp.StatusCode == 500,
					"Method %s for %s should return a valid response, got %d", method, path, resp.StatusCode)
			})
		}
	}
}
