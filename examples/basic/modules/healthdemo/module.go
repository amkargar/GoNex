package healthdemo

import (
	"context"
	"github.com/amkargar/gonex/app"
	transport "github.com/amkargar/gonex/http"
)

type Module struct{}

func (Module) Register(r *app.Registrar) error {
	return transport.Register(r, transport.Endpoint[transport.NoBody, Response]{Method: "GET", Path: "/demo", OperationID: "demo", Status: 200, Handler: func(context.Context, transport.NoBody) (Response, error) { return Response{Message: "Gonex"}, nil }})
}

type Response struct {
	Message string `json:"message"`
}
