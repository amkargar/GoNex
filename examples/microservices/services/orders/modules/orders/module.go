package orders

import (
	"context"
	"github.com/amkargar/gonex/app"
	ordershttp "github.com/amkargar/gonex/examples/microservices/services/orders/modules/orders/http"
)

type Module struct{ Service Service }

func (m Module) Register(r *app.Registrar) error {
	return ordershttp.Register(r, func(ctx context.Context, id string) (ordershttp.Response, error) {
		user, err := m.Service.User(ctx, id)
		return ordershttp.Response{ID: user.ID, Name: user.Name, Email: user.Email}, err
	})
}
