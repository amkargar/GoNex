package users

import "context"

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }
func (s *Service) Create(ctx context.Context, name, email string) (User, error) {
	return s.repository.Create(ctx, name, email)
}
func (s *Service) Get(ctx context.Context, id string) (User, error) { return s.repository.Get(ctx, id) }
