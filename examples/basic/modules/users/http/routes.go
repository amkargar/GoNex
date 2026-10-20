package usershttp

import (
	"github.com/amkargar/gonex/app"
	transport "github.com/amkargar/gonex/http"
)

func Register(r *app.Registrar, actions Actions) error {
	h := Handler{Actions: actions}
	return transport.Routes(r,
		transport.Post("/users", h.Create).
			Name("createUser").
			Summary(createSummary).
			Tags("users").
			Errors(409, 503),
		transport.Get("/users/:id", h.Get).
			Name("getUser").
			Summary(getSummary).
			Tags("users").
			Errors(404, 503),
	)
}
