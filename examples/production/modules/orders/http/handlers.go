package ordershttp

import (
	"context"
	transport "github.com/amkargar/gonex/http"
)

type Actions struct {
	Create func(context.Context, string, int) (Response, error)
	Get    func(context.Context, string) (Response, error)
	List   func(context.Context, int, int, string) ([]Response, error)
	Update func(context.Context, string, string, int, string) (Response, error)
	Delete func(context.Context, string) error
}
type Handler struct{ Actions Actions }

func (h Handler) Create(ctx context.Context, in CreateRequest) (Response, error) {
	return h.Actions.Create(ctx, in.Body.Item, in.Body.Quantity)
}
func (h Handler) Get(ctx context.Context, in GetRequest) (Response, error) {
	return h.Actions.Get(ctx, in.ID)
}
func (h Handler) List(ctx context.Context, in ListRequest) (ListResponse, error) {
	limit, offset, status := 20, 0, ""
	if in.Limit != nil {
		limit = *in.Limit
	}
	if in.Offset != nil {
		offset = *in.Offset
	}
	if in.Status != nil {
		status = *in.Status
	}
	items, err := h.Actions.List(ctx, limit, offset, status)
	return ListResponse{Items: items, Limit: limit, Offset: offset}, err
}
func (h Handler) Update(ctx context.Context, in UpdateRequest) (Response, error) {
	return h.Actions.Update(ctx, in.ID, in.Body.Item, in.Body.Quantity, in.Body.Status)
}
func (h Handler) Delete(ctx context.Context, in GetRequest) (transport.NoBody, error) {
	return transport.NoBody{}, h.Actions.Delete(ctx, in.ID)
}
