package observability

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrettyOutputIsReadableAndKeepsCorrelation(t *testing.T) {
	var out bytes.Buffer
	logger, err := NewLogger(LogOptions{Output: &out, Format: "pretty", Color: "never"})
	if err != nil {
		t.Fatal(err)
	}
	logger.With("service", "shop", "environment", "development").Info("Gonex API server ready",
		"name", "shop", "version", "test-version", "url", "http://localhost:8080",
		"listen", "127.0.0.1:8080", "openapi", "http://localhost:8080/openapi.json", "stop", "Ctrl+C")
	logger.With("request_id", "req-123", "trace_id", "trace-123").Info("HTTP request",
		"method", "GET", "path", "/users/:id", "status", 200, "duration_ms", 2.4)
	for _, want := range []string{"GONEX", "shop", "Ready", "Local", "Listening", "Environment", "Version", "test-version", "Ctrl+C", "GET", "/users/:id", "200", "2.4 ms", "request_id=req-123", "trace_id=trace-123"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal("missing", want, out.String())
		}
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("plain output contains ANSI escapes")
	}
	out.Reset()
	logger.Info("Gonex API server ready", "name", "shop\nINJECTED\x1b[31m", "version", "dev")
	if strings.Contains(out.String(), "\nINJECTED") || strings.Contains(out.String(), "\x1b") {
		t.Fatal("startup field injected a terminal control sequence", out.String())
	}
}
