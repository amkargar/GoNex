package http_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	apperrors "github.com/amkargar/gonex/errors"
	transport "github.com/amkargar/gonex/http"
	"github.com/gin-gonic/gin"
)

type module func(*app.Registrar) error

func (m module) Register(r *app.Registrar) error { return m(r) }

type createBody struct {
	Name string `json:"name" validate:"required,min=2"`
}
type createRequest struct {
	Body createBody `body:"true"`
}
type getRequest struct {
	ID string `path:"id" validate:"required"`
}
type response struct {
	Name string `json:"name"`
}

func newApp(t *testing.T) *app.App {
	t.Helper()
	a, err := app.New(config.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRouteDeclarationsPreserveTypedBehavior(t *testing.T) {
	a := newApp(t)
	middlewareCalls := 0
	if err := a.Register(module(func(r *app.Registrar) error {
		return transport.Routes(r,
			transport.Post("/users", func(_ context.Context, in createRequest) (response, error) {
				if in.Body.Name == "taken" {
					return response{}, apperrors.New(apperrors.Conflict, "name already exists")
				}
				return response{Name: in.Body.Name}, nil
			}).Name("createUser").Summary("Create user").Tags("users").Errors(409).
				Use(func(c *gin.Context) { middlewareCalls++; c.Next() }),
			transport.Get("/users/:id", func(_ context.Context, in getRequest) (response, error) {
				return response{Name: in.ID}, nil
			}).Name("getUser"),
			transport.Delete("/users/:id", func(context.Context, getRequest) (transport.NoBody, error) {
				return transport.NoBody{}, nil
			}).Name("deleteUser"),
		)
	})); err != nil {
		t.Fatal(err)
	}
	handler, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{"POST", "/users", `{"name":"Ada"}`, 201},
		{"POST", "/users", `{"name":"a"}`, 422},
		{"POST", "/users", `{"name":"taken"}`, 409},
		{"POST", "/users", `{"name":`, 400},
		{"GET", "/users/123", "", 200},
		{"DELETE", "/users/123", "", 204},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tc.status || (tc.status == 204 && w.Body.Len() != 0) {
			t.Fatal(tc, w.Code, w.Body.String())
		}
	}
	if middlewareCalls != 4 {
		t.Fatal("middleware did not run before binding and validation", middlewareCalls)
	}
	doc, err := a.OpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Paths map[string]map[string]struct {
			OperationID string         `json:"operationId"`
			Responses   map[string]any `json:"responses"`
		} `json:"paths"`
	}
	if err = json.Unmarshal(doc, &contract); err != nil {
		t.Fatal(err)
	}
	if contract.Paths["/users"]["post"].OperationID != "createUser" || contract.Paths["/users"]["post"].Responses["201"] == nil || contract.Paths["/users"]["post"].Responses["409"] == nil {
		t.Fatal(string(doc))
	}
}

func TestRouteDeclarationsUseVersionedRegistrar(t *testing.T) {
	a := newApp(t)
	if err := a.Register(module(func(r *app.Registrar) error {
		v, err := transport.WithVersion(r, "v2", "/api/v2")
		if err != nil {
			return err
		}
		return transport.Routes(v, transport.Get("/users/:id", func(_ context.Context, in getRequest) (response, error) {
			return response{Name: in.ID}, nil
		}).Name("getUser").Status(202))
	})); err != nil {
		t.Fatal(err)
	}
	doc, err := a.OpenAPIVersion("v2")
	if err != nil || !strings.Contains(string(doc), "/api/v2/users/{id}") || !strings.Contains(string(doc), "getUser_v2") {
		t.Fatal(err, string(doc))
	}
}

func TestInvalidDeclarationsDoNotCommitModule(t *testing.T) {
	a := newApp(t)
	get := func(context.Context, transport.NoBody) (response, error) { return response{Name: "ok"}, nil }
	if err := a.Register(module(func(r *app.Registrar) error {
		return transport.Routes(r,
			transport.Get("/duplicate", get).Name("first"),
			transport.Get("/duplicate", get).Name("second"),
		)
	})); err == nil {
		t.Fatal("duplicate route accepted")
	}
	doc, err := a.OpenAPI()
	if err != nil || strings.Contains(string(doc), "/duplicate") {
		t.Fatal("failed module partially committed", err, string(doc))
	}
	for _, definition := range []transport.RouteDefinition{nil, (*transport.TypedRoute[transport.NoBody, response])(nil)} {
		if err := a.Register(module(func(r *app.Registrar) error { return transport.Routes(r, definition) })); err == nil {
			t.Fatal("nil route accepted")
		}
	}
}
