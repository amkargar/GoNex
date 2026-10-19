package users

import (
	"context"
	"fmt"
	apperrors "github.com/amkargar/gonex/errors"
	"sync"
)

type User struct{ ID, Name, Email string }
type Service struct {
	mu    sync.RWMutex
	users map[string]User
	next  int
}

func NewService() *Service {
	return &Service{users: map[string]User{"1": {ID: "1", Name: "Ada", Email: "ada@example.com"}}, next: 2}
}
func (s *Service) Create(ctx context.Context, name, email string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.Email == email {
			return User{}, apperrors.New(apperrors.Conflict, "email already exists")
		}
	}
	u := User{ID: fmt.Sprint(s.next), Name: name, Email: email}
	s.next++
	s.users[u.ID] = u
	return u, nil
}
func (s *Service) Get(ctx context.Context, id string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, apperrors.New(apperrors.NotFound, "user not found")
	}
	return u, nil
}
