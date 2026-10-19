package http

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"time"
)

// Deadline bounds the handler context; application code must honor cancellation.
func Deadline(timeout time.Duration) (gin.HandlerFunc, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("HTTP deadline must be positive")
	}
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}, nil
}
