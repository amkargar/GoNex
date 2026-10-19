package orders

import "context"

type User struct{ ID, Name, Email string }
type UserReader interface {
	GetUser(context.Context, string) (User, error)
}
type Service struct{ Users UserReader }

func (s Service) User(ctx context.Context, id string) (User, error) { return s.Users.GetUser(ctx, id) }
