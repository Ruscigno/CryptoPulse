package endpoint

import (
	"context"
	"errors"

	"github.com/Ruscigno/CryptoPulse/pkg/service"
	"github.com/go-kit/kit/endpoint"
)

// Endpoints holds all Go-Kit endpoints.
type Endpoints struct {
	PlaceOrder      endpoint.Endpoint
	CancelOrder     endpoint.Endpoint
	GetPositions    endpoint.Endpoint
	ClosePosition   endpoint.Endpoint
	GetOrderStatus  endpoint.Endpoint
	GetOrderHistory endpoint.Endpoint
	CheckHealth     endpoint.Endpoint
}

// MakeEndpoints creates endpoints for the service.
func MakeEndpoints(s service.Service) Endpoints {
	return Endpoints{
		PlaceOrder:      makePlaceOrderEndpoint(s),
		CancelOrder:     makeCancelOrderEndpoint(s),
		GetPositions:    makeGetPositionsEndpoint(s),
		ClosePosition:   makeClosePositionEndpoint(s),
		GetOrderStatus:  makeGetOrderStatusEndpoint(s),
		GetOrderHistory: makeGetOrderHistoryEndpoint(s),
		CheckHealth:     makeCheckHealthEndpoint(s),
	}
}

func makePlaceOrderEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(service.OrderRequest)
		if !ok {
			return nil, errors.New("invalid request")
		}
		return s.PlaceOrder(ctx, req)
	}
}

func makeCancelOrderEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(service.CancelOrderRequest)
		if !ok {
			return nil, errors.New("invalid request")
		}
		return s.CancelOrder(ctx, req)
	}
}

func makeGetPositionsEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		return s.GetPositions(ctx)
	}
}

func makeClosePositionEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(service.ClosePositionRequest)
		if !ok {
			return nil, errors.New("invalid request")
		}
		return s.ClosePosition(ctx, req)
	}
}

func makeGetOrderStatusEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(string)
		if !ok {
			return nil, errors.New("invalid request")
		}
		return s.GetOrderStatus(ctx, req)
	}
}

func makeGetOrderHistoryEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(service.OrderHistoryRequest)
		if !ok {
			return nil, errors.New("invalid request")
		}
		return s.GetOrderHistory(ctx, req)
	}
}

func makeCheckHealthEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		return s.CheckHealth(ctx)
	}
}
