package tests

import (
	"bytes"
	"context"
	"errors"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/observability"
	"github.com/amkargar/gonex/security"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthCORSAndPanicRedaction(t *testing.T) {
	cors, e := security.CORS(security.CORSOptions{Origins: []string{"https://example.com"}, Methods: []string{"GET"}, Headers: []string{"Authorization"}, AllowCredentials: true})
	if e != nil {
		t.Fatal(e)
	}
	metrics := observability.NewMetrics()
	a, e := app.New(config.Config{Middleware: []gin.HandlerFunc{observability.HTTP(metrics), cors}})
	if e != nil {
		t.Fatal(e)
	}
	verify := security.Authenticate(func(_ context.Context, r *http.Request) (security.Principal, error) {
		switch r.Header.Get("Authorization") {
		case "Bearer admin":
			return security.Principal{Subject: "admin", Roles: []string{"admin"}}, nil
		case "Bearer reader":
			return security.Principal{Subject: "reader"}, nil
		}
		return security.Principal{}, errors.New("secret token invalid")
	})
	authorize := security.Authorize(func(p security.Principal, _ *http.Request) bool { return p.Subject == "admin" })
	if e = a.Register(moduleFunc(func(r *app.Registrar) error {
		if e := transport.Register(r, transport.Endpoint[transport.NoBody, transport.NoBody]{Method: "GET", Path: "/private", OperationID: "private", Status: 204, Errors: []int{401, 403}, Middleware: []gin.HandlerFunc{verify, authorize}, Handler: func(context.Context, transport.NoBody) (transport.NoBody, error) { return transport.NoBody{}, nil }}); e != nil {
			return e
		}
		return r.Handle("GET", "/panic", func(*gin.Context) { panic("password=supersecret") })
	})); e != nil {
		t.Fatal(e)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {"Bearer reader", 403}, {"Bearer admin", 204}} {
		w := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/private", nil)
		request.Header.Set("Authorization", tc.token)
		h.ServeHTTP(w, request)
		if w.Code != tc.status || strings.Contains(w.Body.String(), "secret") {
			t.Fatal(tc, w.Code, w.Body)
		}
	}
	w := httptest.NewRecorder()
	request := httptest.NewRequest("OPTIONS", "/private", nil)
	request.Header.Set("Origin", "https://example.com")
	request.Header.Set("Access-Control-Request-Method", "GET")
	request.Header.Set("Access-Control-Request-Headers", "Authorization")
	h.ServeHTTP(w, request)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Fatal(w)
	}
	for _, origin := range []string{"https://evil.example", "null"} {
		w = httptest.NewRecorder()
		request = httptest.NewRequest("GET", "/private", nil)
		request.Header.Set("Origin", origin)
		h.ServeHTTP(w, request)
		if w.Code != 403 || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal(w)
		}
	}
	var logs bytes.Buffer
	old := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logs
	defer func() { gin.DefaultErrorWriter = old }()
	w = httptest.NewRecorder()
	request = httptest.NewRequest("GET", "/panic?secret=hidden", nil)
	request.Header.Set("Authorization", "supersecret")
	request.Header.Set("Cookie", "session=supersecret")
	h.ServeHTTP(w, request)
	if w.Code != 500 || strings.Contains(w.Body.String(), "secret") || strings.Contains(logs.String(), "secret") {
		t.Fatal(w.Code, w.Body, logs.String())
	}
	if metrics.Snapshot().Operations["HTTP GET /panic"].Errors != 1 {
		t.Fatal("panic was not counted")
	}
	if _, e = security.CORS(security.CORSOptions{Origins: []string{"*"}, AllowCredentials: true}); e == nil {
		t.Fatal("wildcard credentials accepted")
	}
}
