// Package security provides explicit transport-boundary middleware primitives.
package security

import (
	"context"
	transport "github.com/amkargar/gonex/http"
	"github.com/gin-gonic/gin"
	nethttp "net/http"
)

type Principal struct {
	Subject string
	Roles   []string
}
type principalKey struct{}

func Current(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

// Authenticate delegates credential extraction and verification to the caller;
// it imposes no JWT, session or bearer-token scheme.
func Authenticate(verify func(context.Context, *nethttp.Request) (Principal, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if verify == nil {
			c.AbortWithStatusJSON(500, transport.ErrorResponse{Code: "internal", Message: "authentication not configured"})
			return
		}
		principal, err := verify(c.Request.Context(), c.Request)
		if err != nil || principal.Subject == "" {
			c.AbortWithStatusJSON(401, transport.ErrorResponse{Code: "unauthorized", Message: "authentication required"})
			return
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), principalKey{}, principal))
		c.Next()
	}
}
func Authorize(allow func(Principal, *nethttp.Request) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := Current(c.Request.Context())
		if !ok {
			c.AbortWithStatusJSON(401, transport.ErrorResponse{Code: "unauthorized", Message: "authentication required"})
			return
		}
		if allow == nil || !allow(principal, c.Request) {
			c.AbortWithStatusJSON(403, transport.ErrorResponse{Code: "forbidden", Message: "access denied"})
			return
		}
		c.Next()
	}
}
