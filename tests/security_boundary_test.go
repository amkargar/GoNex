package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amkargar/gonex/app"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/security"
	"github.com/gin-gonic/gin"
)

func TestAmbiguousOriginsAndPrincipals(t *testing.T) {
	cors, err := security.CORS(security.CORSOptions{Origins: []string{"https://good.example"}, Methods: []string{"GET"}})
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(t)
	if err = a.Register(moduleFunc(func(r *app.Registrar) error {
		return r.Handle("GET", "/boundary", cors, security.Authenticate(func(context.Context, *http.Request) (security.Principal, error) {
			return security.Principal{Subject: " \t"}, nil
		}), func(c *gin.Context) { c.Status(204) })
	})); err != nil {
		t.Fatal(err)
	}
	h, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for _, origins := range [][]string{{"https://good.example", "https://bad.example"}, {"", "https://good.example"}, {"null"}} {
		r := httptest.NewRequest("GET", "/boundary", nil)
		r.Header["Origin"] = origins
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(origins, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/boundary", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("empty identity accepted", w.Code)
	}
}
func TestPrincipalRolesAreDetached(t *testing.T) {
	roles := []string{"reader"}
	a := newApp(t)
	err := a.Register(moduleFunc(func(r *app.Registrar) error {
		return r.Handle("GET", "/roles", security.Authenticate(func(context.Context, *http.Request) (security.Principal, error) {
			return security.Principal{Subject: "user", Roles: roles}, nil
		}), func(c *gin.Context) {
			p, _ := security.Current(c.Request.Context())
			p.Roles[0] = "admin"
			current, _ := security.Current(c.Request.Context())
			if current.Roles[0] != "reader" || roles[0] != "reader" {
				t.Error("roles aliased")
			}
			c.Status(204)
		})
	}))
	if err != nil {
		t.Fatal(err)
	}
	h, _ := a.Handler()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/roles", nil))
}

type boundedInput struct {
	Body struct {
		Name string `json:"name"`
	} `body:"true"`
}

func TestAdversarialJSONBounds(t *testing.T) {
	a := newApp(t)
	if err := a.Register(moduleFunc(func(r *app.Registrar) error {
		return transport.Register(r, transport.Endpoint[boundedInput, transport.NoBody]{Method: "POST", Path: "/input", OperationID: "bounded", Status: 200, MaxBodyBytes: 256, Handler: func(context.Context, boundedInput) (transport.NoBody, error) { return transport.NoBody{}, nil }})
	})); err != nil {
		t.Fatal(err)
	}
	h, _ := a.Handler()
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"name":"` + strings.Repeat("x", 300) + `"}`, 413}, {`{"name":"x","name":"y"}`, 400}, {`{"name":"x","\u006eame":"y"}`, 400}, {string([]byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 255, '"', '}'}), 400}, {strings.Repeat("[", 80) + "0" + strings.Repeat("]", 80), 400}} {
		r := httptest.NewRequest("POST", "/input", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal(w.Code, tc.status, w.Body)
		}
	}
}
