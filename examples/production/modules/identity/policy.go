// Package identity implements a small opaque-token deployment adapter, not an identity provider.
package identity

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/amkargar/gonex/security"
	"github.com/gin-gonic/gin"
)

type Policy struct{ admin, reader string }

func New(admin, reader string) (Policy, error) {
	if len(admin) < 32 || len(reader) < 32 || admin == reader {
		return Policy{}, fmt.Errorf("two distinct API tokens of at least 32 bytes are required")
	}
	return Policy{admin, reader}, nil
}
func (p Policy) Authenticate() gin.HandlerFunc {
	return security.Authenticate(func(_ context.Context, r *http.Request) (security.Principal, error) {
		if len(p.admin) < 32 || len(p.reader) < 32 {
			return security.Principal{}, fmt.Errorf("authentication not configured")
		}
		if len(r.Header.Values("Authorization")) != 1 || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			return security.Principal{}, fmt.Errorf("invalid credential")
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(p.admin)) == 1 {
			return security.Principal{Subject: "api-admin", Roles: []string{"admin"}}, nil
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(p.reader)) == 1 {
			return security.Principal{Subject: "api-reader", Roles: []string{"reader"}}, nil
		}
		return security.Principal{}, fmt.Errorf("invalid credential")
	})
}
func (Policy) Write() gin.HandlerFunc {
	return security.Authorize(func(p security.Principal, _ *http.Request) bool {
		for _, role := range p.Roles {
			if role == "admin" {
				return true
			}
		}
		return false
	})
}
