package users

import (
	"context"
	"github.com/amkargar/gonex/app"
	usershttp "github.com/amkargar/gonex/examples/basic/modules/users/http"
)

type Module struct{}

func (Module) Register(r *app.Registrar) error {
	s := NewService()
	mapUser := func(u User) usershttp.UserResponse {
		return usershttp.UserResponse{ID: u.ID, Name: u.Name, Email: u.Email}
	}
	return usershttp.Register(r, usershttp.Actions{
		Create: func(ctx context.Context, name, email string) (usershttp.UserResponse, error) {
			u, e := s.Create(ctx, name, email)
			return mapUser(u), e
		},
		Get: func(ctx context.Context, id string) (usershttp.UserResponse, error) {
			u, e := s.Get(ctx, id)
			return mapUser(u), e
		},
	})
}
