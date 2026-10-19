package tests

import (
	"context"
	"github.com/amkargar/gonex/app"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/openapi"
	"net/http/httptest"
	"strings"
	"testing"
)

type versionResponse struct {
	Message string `json:"message"`
}
type versionTwoResponse struct {
	Value string `json:"value"`
}

func TestVersionCoexistence(t *testing.T) {
	a := newApp(t)
	calls := 0
	err := a.Register(moduleFunc(func(r *app.Registrar) error {
		register := func(r transport.EndpointRegistrar) error {
			return transport.Register(r, transport.Endpoint[transport.NoBody, versionResponse]{Method: "GET", Path: "/users", OperationID: "users", Status: 200, Handler: func(context.Context, transport.NoBody) (versionResponse, error) {
				calls++
				return versionResponse{Message: "shared"}, nil
			}})
		}
		if e := register(r); e != nil {
			return e
		}
		v1, e := transport.WithVersion(r, "v1", "")
		if e != nil {
			return e
		}
		if e = register(v1); e != nil {
			return e
		}
		v2, e := transport.WithVersion(r, "v2", "")
		if e != nil {
			return e
		}
		return transport.Register(v2, transport.Endpoint[transport.NoBody, versionTwoResponse]{Method: "GET", Path: "/users", OperationID: "users", Status: 200, Handler: func(context.Context, transport.NoBody) (versionTwoResponse, error) {
			calls++
			return versionTwoResponse{Value: "shared"}, nil
		}})
	}))
	if err != nil {
		t.Fatal(err)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/users", "/api/v1/users", "/api/v2/users"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatal(path, w.Code)
		}
	}
	if calls != 3 {
		t.Fatal(calls)
	}
	for _, version := range []string{"v1", "v2"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/openapi/"+version+".json", nil))
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		if e = openapi.Validate(w.Body.Bytes()); e != nil {
			t.Fatal(e)
		}
		if version == "v1" && strings.Contains(w.Body.String(), `"value"`) {
			t.Fatal("v2 leaked into v1")
		}
	}
	if _, e = transport.WithVersion(rProbe{}, "v0", ""); e == nil {
		t.Fatal("invalid version accepted")
	}
}

type rProbe struct{}

func (rProbe) Endpoint(transport.Route) error { return nil }
