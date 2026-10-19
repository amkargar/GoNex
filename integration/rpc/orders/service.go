package orders

import "context"

type User struct{ ID, Name, Email string }
type UserReader interface {
	GetUser(context.Context, string) (User, error)
}
type Service struct{ Users UserReader }

func (s Service) UserName(ctx context.Context, id string) (string, error) {
	u, err := s.Users.GetUser(ctx, id)
	return u.Name, err
}
