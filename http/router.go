// Package http implements Gonex HTTP transport adapters.
package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/amkargar/gonex/openapi"
	"github.com/gin-gonic/gin"
)

type Route struct {
	Method, Path string
	Handlers     []gin.HandlerFunc
	Contract     *openapi.Contract
}

// Build validates the complete route set before exposing a router.
func Build(routes []Route, middleware ...gin.HandlerFunc) (engine *gin.Engine, err error) {
	defer func() {
		if recover() != nil {
			engine = nil
			err = fmt.Errorf("incompatible HTTP routes")
		}
	}()
	engine = gin.New()
	engine.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		c.AbortWithStatusJSON(500, ErrorResponse{Code: "internal", Message: "internal server error"})
	}))
	for _, handler := range middleware {
		if handler == nil {
			return nil, fmt.Errorf("nil HTTP middleware")
		}
	}
	engine.Use(middleware...)
	if err = engine.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, r := range routes {
		if !validMethod(r.Method) || !strings.HasPrefix(r.Path, "/") || strings.ContainsAny(r.Path, "?#") || len(r.Handlers) == 0 {
			return nil, fmt.Errorf("invalid HTTP route %q %q", r.Method, r.Path)
		}
		for _, h := range r.Handlers {
			if h == nil {
				return nil, fmt.Errorf("nil HTTP handler")
			}
		}
		key := r.Method + " " + r.Path
		if seen[key] {
			return nil, fmt.Errorf("duplicate HTTP route %s", key)
		}
		seen[key] = true
		engine.Handle(r.Method, r.Path, r.Handlers...)
	}
	return engine, nil
}

func validMethod(s string) bool {
	switch s {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return true
	}
	return false
}
