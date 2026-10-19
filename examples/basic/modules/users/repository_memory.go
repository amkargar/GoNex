package users

import (
	"context"
	"fmt"
	apperrors "github.com/amkargar/gonex/errors"
	"sync"
)

type MemoryRepository struct {
	mu    sync.RWMutex
	users map[string]User
	next  int
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{users: map[string]User{"1": {ID: "1", Name: "Ada", Email: "ada@example.com"}}, next: 2}
}
func (r *MemoryRepository) Create(ctx context.Context, name, email string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.Email == email {
			return User{}, apperrors.New(apperrors.Conflict, "email already exists")
		}
	}
	u := User{ID: fmt.Sprint(r.next), Name: name, Email: email}
	r.next++
	r.users[u.ID] = u
	return u, nil
}
func (r *MemoryRepository) Get(ctx context.Context, id string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return User{}, apperrors.New(apperrors.NotFound, "user not found")
	}
	return u, nil
}
