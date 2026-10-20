package orders

import (
	"context"
	"testing"
)

type fakeStore struct {
	Store
	called bool
}

func (f *fakeStore) Create(_ context.Context, item string, quantity int) (Order, error) {
	f.called = true
	return Order{ID: "1", Item: item, Quantity: quantity, Status: "pending"}, nil
}
func TestServiceWithoutHTTPOrDatabase(t *testing.T) {
	store := &fakeStore{}
	s := NewService(store)
	if _, err := s.Create(context.Background(), " ", 0); err == nil || store.called {
		t.Fatal("invalid domain write reached store")
	}
	result, err := s.Create(context.Background(), " Book ", 2)
	if err != nil || !store.called || result.Item != "Book" {
		t.Fatal(result, err)
	}
	for _, f := range []Filter{{Limit: 100}, {Limit: 1, Offset: -1}, {Limit: 1, Status: "invalid"}} {
		if _, err := s.List(context.Background(), f); err == nil {
			t.Fatal("invalid filter accepted")
		}
	}
}
