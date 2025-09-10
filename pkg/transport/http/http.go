package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ruscigno/CryptoPulse/pkg/endpoint"
	"github.com/Ruscigno/CryptoPulse/pkg/middleware"
	"github.com/Ruscigno/CryptoPulse/pkg/service"
	httptransport "github.com/go-kit/kit/transport/http"
	"go.uber.org/zap"
)

// HTTPConfig holds HTTP server configuration
type HTTPConfig struct {
	APIKey            string
	MaxBodySize       int64
	RequestsPerSecond int
	BurstSize         int
	Logger            *zap.Logger
	AllowedOrigins    []string
}

// NewHTTPHandler sets up HTTP handlers for the endpoints with middleware.
func NewHTTPHandler(endpoints endpoint.Endpoints, config HTTPConfig) http.Handler {
	mux := http.NewServeMux()

	// Place Order endpoint
	mux.Handle("/place-order", httptransport.NewServer(
		endpoints.PlaceOrder,
		decodePlaceOrderRequest,
		encodeResponse,
	))

	// Cancel Order endpoint
	mux.Handle("/cancel-order", httptransport.NewServer(
		endpoints.CancelOrder,
		decodeCancelOrderRequest,
		encodeResponse,
	))

	// Get Positions endpoint
	mux.Handle("/positions", httptransport.NewServer(
		endpoints.GetPositions,
		decodeGetPositionsRequest,
		encodeResponse,
	))

	// Close Position endpoint
	mux.Handle("/close-position", httptransport.NewServer(
		endpoints.ClosePosition,
		decodeClosePositionRequest,
		encodeResponse,
	))

	// Get Order Status endpoint
	mux.Handle("/orders/", httptransport.NewServer(
		endpoints.GetOrderStatus,
		decodeGetOrderStatusRequest,
		encodeResponse,
	))

	// Get Order History endpoint
	mux.Handle("/order-history", httptransport.NewServer(
		endpoints.GetOrderHistory,
		decodeGetOrderHistoryRequest,
		encodeResponse,
	))

	// Health Check endpoint (no authentication required)
	mux.Handle("/health", httptransport.NewServer(
		endpoints.CheckHealth,
		decodeHealthRequest,
		encodeResponse,
	))

	// Apply middleware stack
	var handler http.Handler = mux

	// Apply middleware in reverse order (last applied = first executed)
	handler = middleware.ErrorLogging(config.Logger)(handler)
	handler = middleware.RequestLogging(middleware.LoggingConfig{
		Logger:           config.Logger,
		LogRequestBody:   false, // Don't log request bodies for security
		LogResponseBody:  false, // Don't log response bodies for security
		SensitiveHeaders: []string{"authorization", "x-api-key"},
		SensitiveFields:  []string{"password", "secret", "token", "key"},
	})(handler)
	handler = middleware.StructuredLogging(config.Logger)(handler)
	handler = middleware.RequestValidation(middleware.ValidationConfig{
		MaxBodySize: config.MaxBodySize,
		Logger:      config.Logger,
	})(handler)
	handler = middleware.RateLimit(middleware.RateLimitConfig{
		RequestsPerSecond: config.RequestsPerSecond,
		BurstSize:         config.BurstSize,
		Logger:            config.Logger,
	})(handler)
	handler = middleware.APIKeyAuth(middleware.AuthConfig{
		APIKey: config.APIKey,
		Logger: config.Logger,
	})(handler)
	handler = middleware.CORS(config.AllowedOrigins)(handler)
	handler = middleware.SecurityHeaders()(handler)
	handler = middleware.RequestID()(handler)

	return handler
}

func decodePlaceOrderRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req service.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return req, nil
}

func decodeCancelOrderRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req service.CancelOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return req, nil
}

func decodeGetPositionsRequest(_ context.Context, r *http.Request) (interface{}, error) {
	// No request body needed for getting positions
	return nil, nil
}

func decodeClosePositionRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req service.ClosePositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return req, nil
}

func decodeGetOrderStatusRequest(_ context.Context, r *http.Request) (interface{}, error) {
	// Extract order ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/orders/")
	if path == "" {
		return nil, http.ErrMissingFile
	}
	return path, nil
}

func decodeGetOrderHistoryRequest(_ context.Context, r *http.Request) (interface{}, error) {
	req := service.OrderHistoryRequest{}

	// Parse query parameters
	query := r.URL.Query()

	if market := query.Get("market"); market != "" {
		req.Market = &market
	}

	if status := query.Get("status"); status != "" {
		req.Status = &status
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = limit
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = offset
		}
	}

	return req, nil
}

func decodeHealthRequest(_ context.Context, r *http.Request) (interface{}, error) {
	// Health check doesn't need any request parameters
	return nil, nil
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(response)
}
