// Package observability provides trace-context propagation and bounded-cardinality metrics.
package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"strings"
)

type traceKey struct{}

func ValidTraceparent(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) != 4 || parts[0] != "00" || len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return false
	}
	for _, part := range parts[1:] {
		if part != strings.ToLower(part) {
			return false
		}
		if _, err := hex.DecodeString(part); err != nil {
			return false
		}
	}
	return parts[1] != strings.Repeat("0", 32) && parts[2] != strings.Repeat("0", 16)
}

// WithTraceparent creates a child context; invalid incoming headers start a fresh trace.
func WithTraceparent(ctx context.Context, incoming string) context.Context {
	traceID := make([]byte, 16)
	spanID := make([]byte, 8)
	_, _ = rand.Read(traceID)
	_, _ = rand.Read(spanID)
	flags := "01"
	id := hex.EncodeToString(traceID)
	if ValidTraceparent(incoming) {
		parts := strings.Split(incoming, "-")
		id = parts[1]
		flags = parts[3]
	}
	return context.WithValue(ctx, traceKey{}, "00-"+id+"-"+hex.EncodeToString(spanID)+"-"+flags)
}
func Traceparent(ctx context.Context) string {
	value, _ := ctx.Value(traceKey{}).(string)
	return value
}

func HTTP(metrics *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		incoming := ""
		values := c.Request.Header.Values("traceparent")
		if len(values) == 1 {
			incoming = values[0]
		}
		ctx := WithTraceparent(c.Request.Context(), incoming)
		c.Request = c.Request.WithContext(ctx)
		c.Header("traceparent", Traceparent(ctx))
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		finish := func(bool) {}
		if metrics != nil {
			method := c.Request.Method
			switch method {
			case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE", "CONNECT":
			default:
				method = "OTHER"
			}
			finish = metrics.Begin("HTTP " + method + " " + path)
		}
		defer func() {
			if fault := recover(); fault != nil {
				finish(true)
				panic(fault)
			}
			finish(c.Writer.Status() >= 400)
		}()
		c.Next()
	}
}
