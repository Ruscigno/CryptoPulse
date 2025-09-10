package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Ruscigno/CryptoPulse/pkg/endpoint"
	"github.com/Ruscigno/CryptoPulse/pkg/service"
	"github.com/go-kit/kit/transport/http"
)

// NewHTTPHandler sets up HTTP handlers for the endpoints.
func NewHTTPHandler(endpoints endpoint.Endpoints) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/place-order", http.NewServer(
		endpoints.PlaceOrder,
		decodePlaceOrderRequest,
		encodeResponse,
	))
	return mux
}

func decodePlaceOrderRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req service.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return req, nil
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(response)
}
