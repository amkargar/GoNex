package http

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	nethttp "net/http"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"github.com/amkargar/gonex/internal/debugui"
	"github.com/gin-gonic/gin"
)

// DebugConfig enables browser diagnostics only through an explicit debug build.
type DebugConfig struct {
	Name string
	Version string
	OpenAPI bool
}

type debugFailure struct {
	Message string
	Type string
	Stack string
}

const debugEnabledKey = "gonex.debug.enabled"
const debugFailureKey = "gonex.debug.failure"

func recordDebugError(c *gin.Context, err error) {
	if enabled, _ := c.Get(debugEnabledKey); enabled != true || err == nil {
		return
	}
	c.Set(debugFailureKey, debugFailure{Message: clipped(err.Error()), Type: fmt.Sprintf("%T", err), Stack: string(debug.Stack())})
}

func clipped(value string) string {
	const limit = 32 << 10
	if len(value) > limit {
		return value[:limit] + "\n… truncated"
	}
	return value
}

func wantsDebugHTML(request *nethttp.Request) bool {
	htmlQuality, jsonQuality := 0.0, 0.0
	for _, part := range strings.Split(request.Header.Get("Accept"), ",") {
		media, params, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		quality := 1.0
		if value, ok := params["q"]; ok {
			quality, err = strconv.ParseFloat(value, 64)
			if err != nil || quality < 0 || quality > 1 {
				continue
			}
		}
		switch strings.ToLower(media) {
		case "text/html":
			htmlQuality = quality
		case "application/json":
			jsonQuality = quality
		}
	}
	return htmlQuality > 0 && htmlQuality >= jsonQuality
}

func debugMiddleware(options DebugConfig, routes []Route) gin.HandlerFunc {
	list := make([]debugui.Route, 0, len(routes))
	for _, route := range routes {
		list = append(list, debugui.Route{Method: route.Method, Path: route.Path})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Path == list[j].Path {
			return list[i].Method < list[j].Method
		}
		return list[i].Path < list[j].Path
	})
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Accept")
		if !wantsDebugHTML(c.Request) {
			c.Next()
			return
		}
		c.Set(debugEnabledKey, true)
		parent := c.Writer
		writer := &errorPageWriter{ResponseWriter: parent, status: parent.Status(), size: -1}
		c.Writer = writer
		defer func() {
			defer func() { c.Writer = parent }()
			if fault := recover(); fault != nil {
				c.Abort()
				// Once a stream has committed, keep its original response intact.
				if writer.forward || parent.Written() {
					return
				}
				writer.status = 500
				writer.body.Reset()
				c.Set("gonex.error", "internal")
				c.Set(debugFailureKey, debugFailure{Message: clipped(fmt.Sprint(fault)), Type: fmt.Sprintf("panic (%T)", fault), Stack: string(debug.Stack())})
			}
			if writer.forward {
				return
			}
			if writer.status < 400 {
				parent.WriteHeader(writer.status)
				return
			}
			if _, exists := c.Get(debugFailureKey); !exists && len(c.Errors) > 0 {
				recordDebugError(c, c.Errors.Last().Err)
			}
			data := debugui.Page{
				Name: options.Name, Version: options.Version,
				Status: writer.status, Title: nethttp.StatusText(writer.status),
				Method: c.Request.Method, Path: c.Request.URL.Path,
				Matched: c.FullPath(), RequestID: parent.Header().Get("X-Request-ID"),
				Routes: list, OpenAPI: options.OpenAPI,
			}
			if value, exists := c.Get(debugFailureKey); exists {
				if failure, ok := value.(debugFailure); ok {
					data.Message, data.ErrorType, data.Stack = failure.Message, failure.Type, failure.Stack
				}
			}
			if data.Message == "" {
				var response ErrorResponse
				if json.Unmarshal(writer.body.Bytes(), &response) == nil && response.Message != "" {
					data.Message = response.Message
					for _, field := range response.Fields {
						data.Message += "\n" + field.Field + ": " + field.Code
					}
				} else {
					data.Message = clipped(writer.body.String())
				}
			}
			if data.Message == "" {
				data.Message = nethttp.StatusText(writer.status)
			}
			if data.Title == "" {
				data.Title = "Request failed"
			}
			debugui.Render(parent, writer.status, data)
		}()
		c.Next()
	}
}

// errorPageWriter holds only uncommitted error responses. Successful bodies and
// streams pass through immediately; large/flushed errors also pass through.
type errorPageWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
	status int
	size int
	forward bool
}

func (w *errorPageWriter) WriteHeader(status int) {
	if !w.forward && w.size < 0 && status > 0 {
		w.status = status
	}
}

func (w *errorPageWriter) WriteHeaderNow() {
	if w.forward {
		w.ResponseWriter.WriteHeaderNow()
		return
	}
	if w.status < 400 {
		w.forward = true
		w.ResponseWriter.WriteHeader(w.status)
		w.ResponseWriter.WriteHeaderNow()
		return
	}
	if w.size < 0 {
		w.size = 0
	}
}

func (w *errorPageWriter) Write(body []byte) (int, error) {
	if w.forward {
		return w.ResponseWriter.Write(body)
	}
	if w.status < 400 || w.body.Len()+len(body) > 64<<10 {
		w.commit()
		return w.ResponseWriter.Write(body)
	}
	if w.size < 0 {
		w.size = 0
	}
	w.size += len(body)
	return w.body.Write(body)
}

func (w *errorPageWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *errorPageWriter) Status() int {
	if w.forward {
		return w.ResponseWriter.Status()
	}
	return w.status
}

func (w *errorPageWriter) Size() int {
	if w.forward {
		return w.ResponseWriter.Size()
	}
	return w.size
}

func (w *errorPageWriter) Written() bool {
	if w.forward {
		return w.ResponseWriter.Written()
	}
	return w.size >= 0
}

func (w *errorPageWriter) commit() {
	if w.forward {
		return
	}
	w.forward = true
	w.ResponseWriter.WriteHeader(w.status)
	if w.body.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.body.Bytes())
		w.body.Reset()
	}
}

func (w *errorPageWriter) Flush() {
	w.commit()
	w.ResponseWriter.Flush()
}

func (w *errorPageWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.forward = true
	return w.ResponseWriter.Hijack()
}

func (w *errorPageWriter) Unwrap() nethttp.ResponseWriter { return w.ResponseWriter }

func debugFallback(options DebugConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch {
		case c.Request.Method == "GET" && c.Request.URL.Path == "/":
			debugui.Render(c.Writer, 200, debugui.Page{Welcome: true, Name: options.Name, Version: options.Version, OpenAPI: options.OpenAPI})
		case c.Request.Method == "GET" && c.Request.URL.Path == "/_gonex/debug/duck.webp":
			c.Header("Cache-Control", "no-store")
			c.Data(200, "image/webp", debugui.Duck())
		default:
			c.JSON(404, ErrorResponse{Code: "not_found", Message: "No route matches this request."})
		}
	}
}
