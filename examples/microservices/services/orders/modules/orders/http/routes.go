package ordershttp

import (
	"context"
	transport "github.com/amkargar/gonex/http"
)

func Register(r transport.EndpointRegistrar, lookup func(context.Context, string) (Response, error)) error {
	return transport.Register(r, transport.Endpoint[Request, Response]{Method: "GET", Path: "/orders/users/:id", OperationID: "orderUser", Status: 200, Errors: []int{404, 503}, Summary: "Look up an order's user through Identity", Handler: func(ctx context.Context, in Request) (Response, error) { return lookup(ctx, in.ID) }})
}
