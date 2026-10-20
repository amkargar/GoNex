package orders

import (
	"context"
	"example.com/gonex-production/modules/identity"
	ordershttp "example.com/gonex-production/modules/orders/http"
	"github.com/amkargar/gonex/app"
)

type Module struct {
	Service *Service
	Policy  identity.Policy
}

func (m Module) Register(r *app.Registrar) error {
	response := func(o Order) ordershttp.Response {
		return ordershttp.Response{ID: o.ID, Item: o.Item, Quantity: o.Quantity, Status: o.Status, CreatedAt: o.CreatedAt}
	}
	return ordershttp.Register(r, ordershttp.Actions{
		Create: func(ctx context.Context, item string, quantity int) (ordershttp.Response, error) {
			o, e := m.Service.Create(ctx, item, quantity)
			return response(o), e
		},
		Get: func(ctx context.Context, id string) (ordershttp.Response, error) {
			o, e := m.Service.Get(ctx, id)
			return response(o), e
		},
		List: func(ctx context.Context, limit, offset int, status string) ([]ordershttp.Response, error) {
			rows, e := m.Service.List(ctx, Filter{limit, offset, status})
			result := make([]ordershttp.Response, 0, len(rows))
			for _, o := range rows {
				result = append(result, response(o))
			}
			return result, e
		},
		Update: func(ctx context.Context, id, item string, quantity int, status string) (ordershttp.Response, error) {
			o, e := m.Service.Update(ctx, id, item, quantity, status)
			return response(o), e
		},
		Delete: m.Service.Delete,
	}, m.Policy)
}
