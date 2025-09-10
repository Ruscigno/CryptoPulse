package endpoint

import (
	"context"
	"errors"

	"github.com/Ruscigno/CryptoPulse/pkg/service"
	"github.com/go-kit/kit/endpoint"
)

// Endpoints holds all Go-Kit endpoints.
type Endpoints struct {
	PlaceOrder endpoint.Endpoint
}

// MakeEndpoints creates endpoints for the service.
func MakeEndpoints(s service.Service) Endpoints {
	return Endpoints{
		PlaceOrder: makePlaceOrderEndpoint(s),
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
