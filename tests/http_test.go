package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/examples/basic/modules/users"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/openapi"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestUserContracts(t *testing.T) {
	a := newApp(t)
	if e := a.Register(users.Module{}); e != nil {
		t.Fatal(e)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		body     string
		status   int
		contains string
	}{
		{`{"name":"Grace","email":"grace@example.com"}`, 201, `"id":"2"`},
		{`{"name":"Grace","email":"grace@example.com"}`, 409, "conflict"},
		{`{"name":"Grace","email":"bad"}`, 422, "email"},
		{`{"name":"Grace"}`, 422, "required"},
		{`{"name":`, 400, "invalid JSON"},
		{`{"unknown":1}`, 400, "invalid JSON"},
		{`{} {}`, 400, "invalid JSON"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/users", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(w, req)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("%s: %d %s", tc.body, w.Code, w.Body)
		}
		if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
			t.Fatal("content type missing")
		}
	}
	for _, tc := range []struct {
		path   string
		status int
	}{{"/users/2", 200}, {"/users/unknown", 404}, {"/openapi.json", 200}, {"/docs", 404}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status {
			t.Fatal(tc, w.Code)
		}
	}
}

type nested struct {
	Email string `json:"email" validate:"required,email"`
}
type payload struct {
	Count    *int     `json:"count" validate:"required,min=0,max=10"`
	Children []nested `json:"children" validate:"required,min=1,dive"`
}
type mixedInput struct {
	ID    int     `path:"id" validate:"min=1"`
	Page  *int    `query:"page" validate:"omitempty,min=0"`
	Token string  `header:"X-Token" validate:"required"`
	Body  payload `body:"true" validate:"required"`
}
type mixedOutput struct {
	ID    int  `json:"id"`
	Page  *int `json:"page"`
	Count int  `json:"count"`
}

func TestBindingAndNestedValidation(t *testing.T) {
	a := newApp(t)
	if e := a.Register(moduleFunc(func(r *app.Registrar) error {
		return transport.Register(r, transport.Endpoint[mixedInput, mixedOutput]{Method: "POST", Path: "/items/:id", OperationID: "mixed", Status: 200, MaxBodyBytes: 128, Handler: func(_ context.Context, in mixedInput) (mixedOutput, error) {
			return mixedOutput{ID: in.ID, Page: in.Page, Count: *in.Body.Count}, nil
		}})
	})); e != nil {
		t.Fatal(e)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		path, body, token string
		status            int
		contains          string
	}{
		{"/items/1?page=0", `{"count":0,"children":[{"email":"a@b.com"}]}`, "token", 200, `"page":0`},
		{"/items/1", `{"count":0,"children":[{"email":"a@b.com"}]}`, "token", 200, `"page":null`},
		{"/items/1", `{"children":[{"email":"a@b.com"}]}`, "token", 422, "count"},
		{"/items/1", `{"count":0,"children":[{"email":"bad"}]}`, "token", 422, "children[0].email"},
		{"/items/x", `{}`, "token", 400, "invalid path"},
		{"/items/1?page=bad", `{}`, "token", 400, "invalid query"},
		{"/items/1?page=1&page=2", `{}`, "token", 400, "duplicate"},
		{"/items/1", `{}`, "", 400, "missing header"},
		{"/items/1", `{"count":1,"children":[{"email":"` + strings.Repeat("a", 130) + `@b.com"}]}`, "token", 413, "too large"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		if tc.token != "" {
			req.Header.Set("X-Token", tc.token)
		}
		h.ServeHTTP(w, req)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body)
		}
	}
	doc, e := a.OpenAPI()
	if e != nil {
		t.Fatal(e)
	}
	if e = openapi.Validate(doc); e != nil {
		t.Fatal(e)
	}
	var raw map[string]any
	json.Unmarshal(doc, &raw)
	op := raw["paths"].(map[string]any)["/items/{id}"].(map[string]any)["post"].(map[string]any)
	if len(op["parameters"].([]any)) != 3 {
		t.Fatal("parameter projection")
	}
	body, _ := json.Marshal(op["requestBody"])
	if strings.Contains(string(body), "page") || strings.Contains(string(body), "X-Token") {
		t.Fatal("parameters leaked into body")
	}
}

type recursive struct {
	Name string     `json:"name"`
	Next *recursive `json:"next,omitempty"`
}
type recursiveRequest struct {
	Body recursive `body:"true"`
}
type ambiguous struct {
	Value string `query:"x" header:"X"`
}

func TestOpenAPIContracts(t *testing.T) {
	contract := openapi.Contract{Method: "POST", Path: "/nodes", OperationID: "nodes", Status: 200, Request: reflect.TypeFor[recursiveRequest](), Response: reflect.TypeFor[recursive]()}
	first, e := openapi.Generate([]openapi.Contract{contract})
	if e != nil {
		t.Fatal(e)
	}
	if e = openapi.Validate(first); e != nil {
		t.Fatal(e)
	}
	second, e := openapi.Generate([]openapi.Contract{contract})
	if e != nil || !bytes.Equal(first, second) {
		t.Fatal("unstable document")
	}
	if _, e = openapi.Generate([]openapi.Contract{contract, contract}); e == nil {
		t.Fatal("duplicate operation accepted")
	}
	if _, e = openapi.Inputs(reflect.TypeFor[ambiguous]()); e == nil {
		t.Fatal("ambiguous origins accepted")
	}
	if e = openapi.Validate([]byte(`{"openapi":"3.1.0"}`)); e == nil {
		t.Fatal("invalid document passed validator")
	}
}

func TestOpenAPIGolden(t *testing.T) {
	a := newApp(t)
	if e := a.Register(users.Module{}); e != nil {
		t.Fatal(e)
	}
	doc, e := a.OpenAPI()
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join("testdata", "basic.openapi.json")
	if os.Getenv("GONEX_UPDATE_GOLDEN") == "1" {
		if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, doc, 0644); e != nil {
			t.Fatal(e)
		}
	}
	want, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(want, doc) {
		t.Fatal("OpenAPI changed; review schema and update golden explicitly")
	}
}

func TestRedactionAndNoBody(t *testing.T) {
	a := newApp(t)
	e := a.Register(moduleFunc(func(r *app.Registrar) error {
		if e := transport.Register(r, transport.Endpoint[transport.NoBody, transport.NoBody]{Method: "DELETE", Path: "/empty", OperationID: "empty", Status: 204, Handler: func(context.Context, transport.NoBody) (transport.NoBody, error) { return transport.NoBody{}, nil }}); e != nil {
			return e
		}
		return transport.Register(r, transport.Endpoint[transport.NoBody, transport.NoBody]{Method: "GET", Path: "/failure", OperationID: "failure", Status: 200, Handler: func(context.Context, transport.NoBody) (transport.NoBody, error) {
			return transport.NoBody{}, errors.New("secret password")
		}})
	}))
	if e != nil {
		t.Fatal(e)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("DELETE", "/empty", nil))
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal(w)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/failure", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w)
	}
	private, e := app.New(config.Config{DisableOpenAPI: true})
	if e != nil {
		t.Fatal(e)
	}
	ph, _ := private.Handler()
	w = httptest.NewRecorder()
	ph.ServeHTTP(w, httptest.NewRequest("GET", "/openapi.json", nil))
	if w.Code != 404 {
		t.Fatal("disabled docs exposed")
	}
}
