package http

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fuzzPayload struct {
	Name   string `json:"name"`
	Values []int  `json:"values"`
}
type fuzzInput struct {
	Body fuzzPayload `body:"true"`
}
type fuzzOutput struct {
	OK bool `json:"ok"`
}
type fuzzRegistrar struct{ route Route }

func (r *fuzzRegistrar) Endpoint(route Route) error { r.route = route; return nil }
func FuzzStrictBinding(f *testing.F) {
	for _, s := range []string{`{"name":"Ada","values":[1,2]}`, `{"name":"a","name":"b"}`, `{"Name":"a"}`, `null`, `{"name":"\u0000"}`, `{"name":"a"} {}`, string([]byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 255, '"', '}'})} {
		f.Add([]byte(s))
	}
	gin.SetMode(gin.TestMode)
	r := &fuzzRegistrar{}
	if err := Register(r, Endpoint[fuzzInput, fuzzOutput]{Method: "POST", Path: "/fuzz", OperationID: "fuzz", Status: 200, MaxBodyBytes: 4096, Handler: func(context.Context, fuzzInput) (fuzzOutput, error) { return fuzzOutput{true}, nil }}); err != nil {
		f.Fatal(err)
	}
	h, err := Build([]Route{r.route})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 8192 {
			t.Skip()
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/fuzz", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(w, req)
		switch w.Code {
		case 200, 400, 413, 422:
		default:
			t.Fatalf("unexpected status %d", w.Code)
		}
		if len(w.Body.Bytes()) > 4096 {
			t.Fatal("unbounded error response")
		}
	})
}
