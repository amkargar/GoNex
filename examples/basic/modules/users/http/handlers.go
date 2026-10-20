package usershttp

import "context"

// Callbacks keep the core independent of HTTP DTOs and avoid an import cycle.
type Actions struct {
	Create func(context.Context, string, string) (UserResponse, error)
	Get    func(context.Context, string) (UserResponse, error)
}

type Handler struct {
	Actions Actions
}

func (h Handler) Create(ctx context.Context, in CreateRequest) (UserResponse, error) {
	return h.Actions.Create(ctx, in.Body.Name, in.Body.Email)
}

func (h Handler) Get(ctx context.Context, in GetRequest) (UserResponse, error) {
	return h.Actions.Get(ctx, in.ID)
}
