// Package debugui contains development-only pages and embedded branding.
package debugui

import (
	"bytes"
	_ "embed"
	"html/template"
	"net/http"
)

//go:embed duck.webp
var duck []byte

//go:embed page.html
var source string

var page = template.Must(template.New("debug").Parse(source))

type Route struct {
	Method string
	Path string
}

type Page struct {
	Welcome bool
	Name string
	Version string
	Status int
	Title string
	Message string
	ErrorType string
	Stack string
	Method string
	Path string
	Matched string
	RequestID string
	Routes []Route
	OpenAPI bool
}

func Duck() []byte { return duck }

func Render(w http.ResponseWriter, status int, data Page) {
	var body bytes.Buffer
	if err := page.Execute(&body, data); err != nil {
		http.Error(w, "development page unavailable", http.StatusInternalServerError)
		return
	}
	header := w.Header()
	header.Del("Content-Length")
	header.Del("Content-Encoding")
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}
