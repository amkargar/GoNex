package usershttp

import "context"

// Callbacks keep the core independent of HTTP DTOs and avoid an import cycle.
type Actions struct {
	Create func(context.Context, string, string) (UserResponse, error)
	Get    func(context.Context, string) (UserResponse, error)
}
