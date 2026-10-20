package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/observability"
	"github.com/gin-gonic/gin"
)

func logRecords(t *testing.T, b *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(b.String()), "\n") {
		if line == "" {
			continue
		}
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err, line)
		}
		records = append(records, r)
	}
	return records
}

func TestRequestLogsAndContext(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	a, err := app.New(config.Config{Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Register(moduleFunc(func(r *app.Registrar) error {
		if err := r.Handle("GET", "/users/:id", func(c *gin.Context) {
			ctx := c.Request.Context()
			if observability.RequestID(ctx) == "" {
				t.Error("missing request context ID")
			}
			observability.Logger(ctx, nil).InfoContext(ctx, "service event", "module", "users")
			c.JSON(200, gin.H{"ok": true})
		}); err != nil {
			return err
		}
		return r.Handle("GET", "/panic", func(c *gin.Context) { panic("secret panic payload") })
	})); err != nil {
		t.Fatal(err)
	}
	h, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/users/secret-user?token=secret-query", "/missing-secret", "/panic"} {
		output.Reset()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer secret-header")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		records := logRecords(t, &output)
		completed := 0
		for _, r := range records {
			if r["request_id"] != w.Header().Get("X-Request-ID") {
				t.Fatal("request correlation mismatch", r)
			}
			if r["msg"] == "HTTP request" {
				completed++
				if r["status"] != float64(w.Code) {
					t.Fatal(r, w.Code)
				}
				if path == "/panic" && (r["level"] != "ERROR" || r["error"] != "handler panic") {
					t.Fatal(r)
				}
			}
		}
		if completed != 1 {
			t.Fatal("expected one completion", records)
		}
		for _, secret := range []string{"secret-user", "secret-query", "missing-secret", "secret-header", "secret panic payload"} {
			if strings.Contains(output.String(), secret) {
				t.Fatal("secret leaked", output.String())
			}
		}
	}
}

func TestStartupURLsAndFailure(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		var output bytes.Buffer
		a, err := app.New(config.Config{Address: "0.0.0.0:0", DocsEnabled: true, DisableOpenAPI: disabled, Logger: slog.New(slog.NewJSONHandler(&output, nil))})
		if err != nil {
			t.Fatal(err)
		}
		if err = a.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		_, port, _ := net.SplitHostPort(a.Address())
		want := "http://localhost:" + port
		if err = a.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		records := logRecords(t, &output)
		r := records[0]
		if r["msg"] != "Gonex API server ready" || r["url"] != want || r["listen"] != a.Address() {
			t.Fatal(r)
		}
		if disabled {
			if r["openapi"] != nil || r["docs"] != nil {
				t.Fatal(r)
			}
		} else if r["openapi"] != want+"/openapi.json" || r["docs"] != want+"/docs" {
			t.Fatal(r)
		}
		if len(records) != 3 || records[2]["msg"] != "server stopped" {
			t.Fatal(records)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var output bytes.Buffer
	a, err := app.New(config.Config{Address: listener.Addr().String(), Logger: slog.New(slog.NewJSONHandler(&output, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Start(context.Background()); err == nil {
		t.Fatal("address collision accepted")
	}
	if strings.Contains(output.String(), "server ready") {
		t.Fatal("false ready event")
	}
	if records := logRecords(t, &output); len(records) != 1 || records[0]["level"] != "ERROR" {
		t.Fatal(records)
	}
}

func TestLogConfiguration(t *testing.T) {
	for _, format := range []string{"auto", "pretty", "text", "json"} {
		var output bytes.Buffer
		logger, err := observability.NewLogger(observability.LogOptions{Output: &output, Environment: "production", Format: format, Level: "WARN", Color: "auto"})
		if err != nil {
			t.Fatal(err)
		}
		logger.Info("hidden")
		logger.With("module", "users").WithGroup("db").Warn("query failed", "error", fmt.Errorf("line one\nline two"))
		if strings.Contains(output.String(), "hidden") || strings.Contains(output.String(), "\x1b") {
			t.Fatal(output.String())
		}
		if format == "auto" || format == "json" {
			r := logRecords(t, &output)
			if len(r) != 1 || r[0]["level"] != "WARN" || r[0]["module"] != "users" {
				t.Fatal(r)
			}
		} else if strings.Count(output.String(), "\n") != 1 {
			t.Fatal("multiline injection", output.String())
		}
	}
	for _, c := range []config.Config{{LogLevel: "bad"}, {LogFormat: "bad"}, {LogColor: "bad"}} {
		if _, err := app.New(c); err == nil {
			t.Fatal("invalid logging config accepted")
		}
	}
}
