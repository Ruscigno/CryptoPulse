package service

import (
	"context"
	"errors"
)

// OrderRequest defines the input for placing an order.
type OrderRequest struct {
	Market string  `json:"market"`
	Side   string  `json:"side"`
	Size   float64 `json:"size"`
}

// OrderResponse defines the response for an order operation.
type OrderResponse struct {
	OrderID string `json:"orderId"`
}

// Service defines the order routing service interface.
type Service interface {
	PlaceOrder(ctx context.Context, req OrderRequest) (OrderResponse, error)
}

// service implements the Service interface.
type service struct{}

// NewService creates a new Service instance.
func NewService() Service {
	return &service{}
}

func (s *service) PlaceOrder(ctx context.Context, req OrderRequest) (OrderResponse, error) {
	// Placeholder: Integrate with wallet and tx packages later
	if req.Market == "" || req.Side == "" || req.Size <= 0 {
		return OrderResponse{}, errors.New("invalid order parameters")
	}
	// Simulate order placement
	return OrderResponse{OrderID: "order-123"}, nil
}
