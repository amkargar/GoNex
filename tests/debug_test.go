package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	transport "github.com/amkargar/gonex/http"
	"github.com/gin-gonic/gin"
)

type debugReply struct {
	Message string `json:"message"`
}
type debugBody struct {
	Name string `json:"name" validate:"required,min=2"`
}
type debugRequest struct {
	Body debugBody `body:"true"`
}

func debugApp(t *testing.T, enabled, customRoot, disableOpenAPI bool) *app.App {
	t.Helper()
	a, err := app.New(config.Config{
		Debug: enabled, DisableOpenAPI: disableOpenAPI,
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Register(moduleFunc(func(r *app.Registrar) error {
		if customRoot {
			if err := r.Handle("GET", "/", func(c *gin.Context) { c.String(200, "my application") }); err != nil {
				return err
			}
		}
		if err := r.Handle("GET", "/panic", func(*gin.Context) {
			panic("debug panic <script>alert(1)</script>")
		}); err != nil {
			return err
		}
		if err := r.Handle("GET", "/stream", func(c *gin.Context) {
			c.Header("Content-Type", "text/event-stream")
			c.Writer.WriteString("data: ready\n\n")
			c.Writer.Flush()
		}); err != nil {
			return err
		}
		if err := r.Handle("GET", "/large-error", func(c *gin.Context) {
			c.String(500, strings.Repeat("x", 70<<10))
		}); err != nil {
			return err
		}
		if err := r.Handle("GET", "/raw-error", func(c *gin.Context) {
			_ = c.Error(fmt.Errorf("raw route cause"))
			c.AbortWithStatus(500)
		}); err != nil {
			return err
		}
		return transport.Routes(r,
			transport.Get("/error", func(context.Context, transport.NoBody) (debugReply, error) {
				return debugReply{}, fmt.Errorf("private handler error <script>alert(2)</script>")
			}).Name("debugError"),
			transport.Post("/validate", func(_ context.Context, in debugRequest) (debugReply, error) {
				return debugReply{Message: in.Body.Name}, nil
			}).Name("debugValidate"),
		)
	})); err != nil {
		t.Fatal(err)
	}
	return a
}

func debugCall(t *testing.T, a *app.App, method, path, accept, body string) *httptest.ResponseRecorder {
	t.Helper()
	h, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Authorization", "Bearer HEADER_SECRET")
	req.Header.Set("Cookie", "private=COOKIE_SECRET")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestDebugWelcomeAndEmbeddedDuck(t *testing.T) {
	a := debugApp(t, true, false, false)
	w := debugCall(t, a, "GET", "/", "text/html", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Welcome to Gonex") || !strings.Contains(w.Body.String(), "/_gonex/debug/duck.webp") || !strings.Contains(w.Body.String(), "GoNex-website/docs/main/") {
		t.Fatal(w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal(w.Header())
	}
	duck := debugCall(t, a, "GET", "/_gonex/debug/duck.webp", "", "")
	if duck.Code != 200 || duck.Header().Get("Content-Type") != "image/webp" || !strings.HasPrefix(duck.Body.String(), "RIFF") {
		t.Fatal("embedded image unavailable", duck.Code, duck.Header())
	}
	own := debugCall(t, debugApp(t, true, true, false), "GET", "/", "text/html", "")
	if own.Body.String() != "my application" {
		t.Fatal("debug welcome replaced user root", own.Body.String())
	}
	noAPI := debugCall(t, debugApp(t, true, false, true), "GET", "/", "text/html", "")
	if strings.Contains(noAPI.Body.String(), "href=\"/openapi.json\"") {
		t.Fatal("disabled OpenAPI advertised")
	}
}

func TestDebugHTMLFailuresAndEscaping(t *testing.T) {
	a := debugApp(t, true, false, false)
	for _, tc := range []struct {
		method string
		path   string
		body   string
		status int
		detail string
	}{
		{"GET", "/error?token=QUERY_SECRET", "", 500, "private handler error"},
		{"GET", "/panic", "", 500, "debug panic"},
		{"GET", "/missing?token=QUERY_SECRET", "", 404, "Registered routes"},
		{"POST", "/error", "", 405, "requested method"},
		{"POST", "/validate", `{"name":"x"}`, 422, "min"},
		{"POST", "/validate", `{"name":`, 400, "invalid JSON body"},
		{"GET", "/raw-error", "", 500, "raw route cause"},
	} {
		w := debugCall(t, a, tc.method, tc.path, "text/html,application/xhtml+xml", tc.body)
		if w.Code != tc.status || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") || !strings.Contains(w.Body.String(), tc.detail) {
			t.Fatal(tc, w.Code, w.Body.String())
		}
		for _, secret := range []string{"QUERY_SECRET", "HEADER_SECRET", "COOKIE_SECRET", "<script>alert("} {
			if strings.Contains(w.Body.String(), secret) {
				t.Fatal("unescaped HTML or request secret", secret)
			}
		}
		if w.Header().Get("X-Request-ID") == "" || !strings.Contains(w.Body.String(), w.Header().Get("X-Request-ID")) {
			t.Fatal("missing request correlation")
		}
		if (tc.status == 500 && (tc.path == "/panic" || strings.HasPrefix(tc.path, "/error"))) && (!strings.Contains(w.Body.String(), "Stack trace") || !strings.Contains(w.Body.String(), "&lt;script&gt;")) {
			t.Fatal("missing escaped exception/stack", w.Body.String())
		}
	}
}

func TestDebugOffAndJSONNeverExposeDetails(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		a := debugApp(t, enabled, false, false)
		for _, accept := range []string{"application/json", "", "text/html;q=0", "text/html;q=0.2,application/json;q=1"} {
			for _, path := range []string{"/error", "/panic"} {
				w := debugCall(t, a, "GET", path, accept, "")
				if w.Code != 500 || !json.Valid(w.Body.Bytes()) || strings.Contains(w.Body.String(), "private handler") || strings.Contains(w.Body.String(), "debug panic") || strings.Contains(w.Body.String(), "Stack trace") {
					t.Fatal(enabled, accept, path, w.Code, w.Body.String())
				}
			}
		}
	}
	a := debugApp(t, false, false, false)
	for _, path := range []string{"/", "/_gonex/debug/duck.webp", "/missing", "/panic", "/error"} {
		w := debugCall(t, a, "GET", path, "text/html", "")
		for _, forbidden := range []string{"Welcome to Gonex", "Stack trace", "private handler", "debug panic", "Registered routes", "DEBUG=true"} {
			if strings.Contains(w.Body.String(), forbidden) {
				t.Fatal("debug content leaked when off", path, forbidden)
			}
		}
		if (path == "/" || path == "/_gonex/debug/duck.webp") && w.Code != 404 {
			t.Fatal("debug-only URL available while disabled", path, w.Code)
		}
	}
}

func TestDebugDoesNotRewriteSuccessOrStreams(t *testing.T) {
	a := debugApp(t, true, false, false)
	success := debugCall(t, a, "POST", "/validate", "text/html", `{"name":"Ada"}`)
	if success.Code != 201 || !json.Valid(success.Body.Bytes()) {
		t.Fatal("successful JSON response changed", success.Code, success.Body.String())
	}
	stream := debugCall(t, a, "GET", "/stream", "text/html", "")
	if stream.Body.String() != "data: ready\n\n" || !stream.Flushed {
		t.Fatal("stream changed", stream.Body.String())
	}
	large := debugCall(t, a, "GET", "/large-error", "text/html", "")
	if large.Code != 500 || large.Body.Len() != 70<<10 || strings.Contains(large.Body.String(), "<html") {
		t.Fatal("large response truncated or rewritten", large.Code, large.Body.Len())
	}
	doc, err := a.OpenAPI()
	if err != nil || strings.Contains(string(doc), "/_gonex/debug/") {
		t.Fatal("debug route leaked into API contract", err)
	}
}
