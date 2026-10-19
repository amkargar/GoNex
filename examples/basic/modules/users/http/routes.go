package usershttp

import (
	"context"
	"github.com/amkargar/gonex/app"
	transport "github.com/amkargar/gonex/http"
)

func Register(r *app.Registrar, a Actions) error {
	if err := transport.Register(r, transport.Endpoint[CreateRequest, UserResponse]{Method: "POST", Path: "/users", OperationID: "createUser", Summary: createSummary, Tags: []string{"users"}, Status: 201, Errors: []int{409}, Handler: func(ctx context.Context, in CreateRequest) (UserResponse, error) {
		return a.Create(ctx, in.Body.Name, in.Body.Email)
	}}); err != nil {
		return err
	}
	return transport.Register(r, transport.Endpoint[GetRequest, UserResponse]{Method: "GET", Path: "/users/:id", OperationID: "getUser", Summary: getSummary, Tags: []string{"users"}, Status: 200, Errors: []int{404}, Handler: func(ctx context.Context, in GetRequest) (UserResponse, error) { return a.Get(ctx, in.ID) }})
}
