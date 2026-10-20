package orders

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "github.com/amkargar/gonex/errors"
)

type Order struct {
	ID, Item, Status string
	Quantity         int
	CreatedAt        time.Time
}
type Filter struct {
	Limit, Offset int
	Status        string
}
type Store interface {
	Create(context.Context, string, int) (Order, error)
	Get(context.Context, string) (Order, error)
	List(context.Context, Filter) ([]Order, error)
	Update(context.Context, string, string, int, string) (Order, error)
	Delete(context.Context, string) error
}
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store} }
func (s *Service) Create(ctx context.Context, item string, quantity int) (Order, error) {
	if err := valid(item, quantity, "pending"); err != nil {
		return Order{}, err
	}
	return s.store.Create(ctx, strings.TrimSpace(item), quantity)
}
func (s *Service) Get(ctx context.Context, id string) (Order, error) { return s.store.Get(ctx, id) }
func (s *Service) List(ctx context.Context, f Filter) ([]Order, error) {
	if f.Limit < 1 || f.Limit > 50 || f.Offset < 0 || f.Offset > 10000 || f.Status != "" && f.Status != "pending" && f.Status != "completed" {
		return nil, apperrors.New(apperrors.Invalid, "invalid pagination or status")
	}
	return s.store.List(ctx, f)
}
func (s *Service) Update(ctx context.Context, id, item string, quantity int, status string) (Order, error) {
	if err := valid(item, quantity, status); err != nil {
		return Order{}, err
	}
	return s.store.Update(ctx, id, strings.TrimSpace(item), quantity, status)
}
func (s *Service) Delete(ctx context.Context, id string) error { return s.store.Delete(ctx, id) }
func valid(item string, quantity int, status string) error {
	if !utf8.ValidString(item) || utf8.RuneCountInString(strings.TrimSpace(item)) < 2 || utf8.RuneCountInString(strings.TrimSpace(item)) > 80 || quantity < 1 || quantity > 1000 || status != "pending" && status != "completed" {
		return apperrors.New(apperrors.Invalid, "invalid order")
	}
	return nil
}
