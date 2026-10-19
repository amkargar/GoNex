package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	apperrors "github.com/amkargar/gonex/errors"
	"github.com/amkargar/gonex/examples/basic/modules/users"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/openapi"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenAPIReleaseCompatibility(t *testing.T) {
	baseline, e := os.ReadFile("testdata/releases/v0.1.0.openapi.json")
	if e != nil {
		t.Fatal(e)
	}
	a := newApp(t)
	if e = a.Register(users.Module{}); e != nil {
		t.Fatal(e)
	}
	current, e := a.OpenAPI()
	if e != nil {
		t.Fatal(e)
	}
	if e = openapi.Compatible(baseline, current); e != nil {
		t.Fatal(e)
	}
	var changed map[string]any
	if e = json.Unmarshal(current, &changed); e != nil {
		t.Fatal(e)
	}
	delete(changed["paths"].(map[string]any), "/users/{id}")
	removed, e := json.Marshal(changed)
	if e != nil {
		t.Fatal(e)
	}
	if e = openapi.Compatible(baseline, removed); e == nil {
		t.Fatal("removed path accepted")
	}
	if e = json.Unmarshal(current, &changed); e != nil {
		t.Fatal(e)
	}
	changed["paths"].(map[string]any)["/users/{id}"].(map[string]any)["get"].(map[string]any)["operationId"] = "renamedUser"
	renamed, _ := json.Marshal(changed)
	if e = openapi.Compatible(baseline, renamed); e == nil {
		t.Fatal("changed operation ID accepted")
	}
	changed = nil
	if e = json.Unmarshal(current, &changed); e != nil {
		t.Fatal(e)
	}
	changed["paths"].(map[string]any)["/users/{id}"].(map[string]any)["parameters"] = []any{map[string]any{
		"name": "X-Required", "in": "header", "required": true, "schema": map[string]any{"type": "string"},
	}}
	parameterAdded, _ := json.Marshal(changed)
	if e = openapi.Compatible(baseline, parameterAdded); e == nil {
		t.Fatal("new required path-level parameter accepted")
	}
}

type toggleBody struct {
	Enabled bool `json:"enabled"`
}
type toggleRequest struct {
	Body toggleBody `body:"true"`
}
type pointerToggleRequest struct {
	Body *toggleBody `body:"true" validate:"required"`
}
type toggleResponse struct {
	Enabled bool `json:"enabled"`
}

func TestZeroBodyAndRequiredPointerPresence(t *testing.T) {
	a := newApp(t)
	if e := a.Register(moduleFunc(func(r *app.Registrar) error {
		if e := transport.Register(r, transport.Endpoint[toggleRequest, toggleResponse]{Method: "POST", Path: "/toggle", OperationID: "toggle", Status: 200, Handler: func(_ context.Context, in toggleRequest) (toggleResponse, error) {
			return toggleResponse{Enabled: in.Body.Enabled}, nil
		}}); e != nil {
			return e
		}
		return transport.Register(r, transport.Endpoint[pointerToggleRequest, toggleResponse]{Method: "POST", Path: "/pointer", OperationID: "pointerToggle", Status: 200, Handler: func(_ context.Context, in pointerToggleRequest) (toggleResponse, error) {
			return toggleResponse{Enabled: in.Body.Enabled}, nil
		}})
	})); e != nil {
		t.Fatal(e)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		path, body string
		status     int
	}{{"/toggle", `{"enabled":false}`, 200}, {"/pointer", `{"enabled":false}`, 200}, {"/toggle", `null`, 400}, {"/pointer", `null`, 422}, {"/toggle", `{"Enabled":false}`, 400}} {
		w := httptest.NewRecorder()
		request := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(w, request)
		if w.Code != tc.status {
			t.Fatal(tc, w.Code, w.Body)
		}
	}
}

func TestRegisteredMetadataAndErrorPolicyAreFrozen(t *testing.T) {
	a := newApp(t)
	tags := []string{"original"}
	errors := []int{404}
	schemes := openapi.Object{"bearerAuth": openapi.Object{"type": "http", "scheme": "bearer"}}
	if e := a.Register(moduleFunc(func(r *app.Registrar) error {
		return transport.Register(r, transport.Endpoint[transport.NoBody, transport.NoBody]{Method: "GET", Path: "/metadata", OperationID: "metadata", Status: 200, Tags: tags, Errors: errors, Security: []map[string][]string{{"bearerAuth": {}}}, SecuritySchemes: schemes, Handler: func(context.Context, transport.NoBody) (transport.NoBody, error) {
			return transport.NoBody{}, apperrors.New(apperrors.NotFound, "not found")
		}})
	})); e != nil {
		t.Fatal(e)
	}
	tags[0] = "modified"
	errors[0] = 409
	schemes["bearerAuth"].(map[string]any)["scheme"] = "basic"
	doc, e := a.OpenAPI()
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(doc), "modified") || !strings.Contains(string(doc), `"scheme": "bearer"`) {
		t.Fatal("metadata retained mutable input")
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/metadata", nil))
	if w.Code != 404 {
		t.Fatal("runtime error policy retained mutable input", w.Code)
	}
}

func TestHTTPDeadline(t *testing.T) {
	deadline, e := transport.Deadline(10 * time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	a, e := app.New(config.Config{Middleware: []gin.HandlerFunc{deadline}})
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Register(moduleFunc(func(r *app.Registrar) error {
		return transport.Register(r, transport.Endpoint[transport.NoBody, traceResponse]{Method: "GET", Path: "/slow", OperationID: "slow", Status: 200, Handler: func(ctx context.Context, _ transport.NoBody) (traceResponse, error) {
			<-ctx.Done()
			return traceResponse{}, ctx.Err()
		}})
	})); e != nil {
		t.Fatal(e)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/slow", nil))
	if w.Code != 504 || !strings.Contains(w.Body.String(), "timeout") {
		t.Fatal(w.Code, w.Body)
	}
	doc, e := a.OpenAPI()
	if e != nil || !bytes.Contains(doc, []byte(`"504"`)) {
		t.Fatal("deadline response undocumented", e)
	}
}
