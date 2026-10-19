package users

import "context"

type User struct{ ID, Name, Email string }

// Repository is defined by the application's actual use cases.
type Repository interface {
	Create(context.Context, string, string) (User, error)
	Get(context.Context, string) (User, error)
}
