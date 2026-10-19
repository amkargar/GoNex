package security

import (
	"fmt"
	transport "github.com/amkargar/gonex/http"
	"github.com/gin-gonic/gin"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type CORSOptions struct {
	Origins, Methods, Headers []string
	AllowCredentials          bool
	MaxAge                    time.Duration
}

func CORS(options CORSOptions) (gin.HandlerFunc, error) {
	options.Methods = append([]string(nil), options.Methods...)
	origins := map[string]bool{}
	methods := map[string]bool{}
	headers := map[string]bool{}
	for _, origin := range options.Origins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Scheme != "https" && parsed.Scheme != "http" {
			return nil, fmt.Errorf("CORS requires explicit HTTP(S) origins")
		}
		origins[origin] = true
	}
	if len(options.Methods) == 0 {
		options.Methods = []string{"GET", "HEAD"}
	}
	for _, method := range options.Methods {
		switch method {
		case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
			methods[method] = true
		default:
			return nil, fmt.Errorf("invalid CORS method")
		}
	}
	for _, header := range options.Headers {
		if !headerToken(header) {
			return nil, fmt.Errorf("invalid CORS header")
		}
		headers[strings.ToLower(header)] = true
	}
	if options.MaxAge < 0 || options.MaxAge > 24*time.Hour {
		return nil, fmt.Errorf("CORS max age must be between zero and 24h")
	}
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		deny := func() {
			c.AbortWithStatusJSON(403, transport.ErrorResponse{Code: "forbidden", Message: "origin or preflight not allowed"})
		}
		if !origins[origin] {
			deny()
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		if options.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == "OPTIONS" && c.GetHeader("Access-Control-Request-Method") != "" {
			c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
			c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
			if !methods[c.GetHeader("Access-Control-Request-Method")] {
				deny()
				return
			}
			requested := c.GetHeader("Access-Control-Request-Headers")
			if requested != "" {
				for _, header := range strings.Split(requested, ",") {
					if !headers[strings.ToLower(strings.TrimSpace(header))] {
						deny()
						return
					}
				}
			}
			c.Header("Access-Control-Allow-Methods", strings.Join(options.Methods, ", "))
			if requested != "" {
				c.Header("Access-Control-Allow-Headers", requested)
			}
			if options.MaxAge > 0 {
				c.Header("Access-Control-Max-Age", strconv.FormatInt(int64(options.MaxAge/time.Second), 10))
			}
			c.AbortWithStatus(204)
			return
		}
		if !methods[c.Request.Method] {
			deny()
			return
		}
		c.Next()
	}, nil
}
func headerToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r) {
			continue
		}
		return false
	}
	return true
}
