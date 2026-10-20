package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LogOptions configures one application logger. No global slog logger is changed.
type LogOptions struct {
	Output                            io.Writer
	Environment, Format, Level, Color string
}

func NewLogger(o LogOptions) (*slog.Logger, error) {
	if o.Output == nil {
		o.Output = os.Stderr
	}
	level := slog.LevelInfo
	if o.Level != "" {
		if err := level.UnmarshalText([]byte(o.Level)); err != nil {
			return nil, fmt.Errorf("invalid log level: %w", err)
		}
	}
	format := o.Format
	if format == "" || format == "auto" {
		format = "pretty"
		if o.Environment == "production" {
			format = "json"
		}
	}
	opts := &slog.HandlerOptions{Level: level}
	switch format {
	case "json":
		return slog.New(slog.NewJSONHandler(o.Output, opts)), nil
	case "text":
		return slog.New(slog.NewTextHandler(o.Output, opts)), nil
	case "pretty":
		color := false
		switch o.Color {
		case "", "auto":
			if f, ok := o.Output.(*os.File); ok {
				if st, err := f.Stat(); err == nil {
					color = st.Mode()&os.ModeCharDevice != 0 && os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == ""
				}
			}
		case "always":
			color = true
		case "never":
		default:
			return nil, fmt.Errorf("log color must be auto, always or never")
		}
		return slog.New(&prettyHandler{out: o.Output, mu: &sync.Mutex{}, level: level, color: color}), nil
	default:
		return nil, fmt.Errorf("log format must be auto, pretty, text or json")
	}
}

type prettyHandler struct {
	out   io.Writer
	mu    *sync.Mutex
	level slog.Level
	color bool
	attrs []slog.Attr
	group string
}

func (h *prettyHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }
func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.attrs = append(append([]slog.Attr(nil), h.attrs...), h.qualify(attrs)...)
	return &c
}
func (h *prettyHandler) qualify(attrs []slog.Attr) []slog.Attr {
	result := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		if h.group != "" {
			a.Key = h.group + "." + a.Key
		}
		result = append(result, a)
	}
	return result
}
func (h *prettyHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	c := *h
	if c.group != "" {
		c.group += "."
	}
	c.group += name
	return &c
}
func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	label := fmt.Sprintf("%-5s", r.Level.String())
	if h.color {
		code := "36"
		if r.Level >= slog.LevelError {
			code = "31"
		} else if r.Level >= slog.LevelWarn {
			code = "33"
		}
		label = "\x1b[" + code + "m" + label + "\x1b[0m"
	}
	fmt.Fprintf(&b, "%s  %s  %s", r.Time.Format("15:04:05.000"), label, strconv.Quote(r.Message))
	attrs := append([]slog.Attr(nil), h.attrs...)
	r.Attrs(func(a slog.Attr) bool { attrs = append(attrs, h.qualify([]slog.Attr{a})...); return true })
	if r.Message == "Gonex API server ready" {
		for _, a := range attrs {
			b.WriteString("\n    ")
			writeAttr(&b, "", a)
		}
	} else {
		for _, a := range attrs {
			writeAttr(&b, "", a)
		}
	}
	b.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, b.String())
	return err
}
func writeAttr(b *strings.Builder, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	key := prefix + a.Key
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			key += "."
		}
		for _, child := range a.Value.Group() {
			writeAttr(b, key, child)
		}
		return
	}
	value := a.Value.String()
	if strings.ContainsAny(value, "\n\r\t\x1b \"") {
		value = strconv.Quote(value)
	}
	fmt.Fprintf(b, "  %s=%s", safeKey(key), value)
}

type loggerKey struct{}
type requestKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// Logger returns the request logger when installed, otherwise the caller's fallback.
func Logger(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	if fallback != nil {
		return fallback
	}
	return slog.Default()
}
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestKey{}).(string)
	return value
}

// RequestLog emits exactly one completion event, including when recovery handles a panic.
// It never records body, headers, query strings, raw unmatched URLs or panic payloads.
func RequestLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		id := make([]byte, 16)
		_, _ = rand.Read(id)
		requestID := hex.EncodeToString(id)
		c.Header("X-Request-ID", requestID)
		requestLogger := logger.With("request_id", requestID)
		incoming := ""
		if values := c.Request.Header.Values("traceparent"); len(values) == 1 {
			incoming = values[0]
		}
		traceCtx := WithTraceparent(c.Request.Context(), incoming)
		trace := Traceparent(traceCtx)
		requestLogger = requestLogger.With("trace_id", strings.Split(trace, "-")[1])
		c.Header("traceparent", trace)
		ctx := context.WithValue(traceCtx, requestKey{}, requestID)
		c.Request = c.Request.WithContext(WithLogger(ctx, requestLogger))
		defer func() {
			fault := recover()
			status := c.Writer.Status()
			level := slog.LevelInfo
			path := c.FullPath()
			if path == "" {
				path = "unmatched"
			}
			if fault != nil && !c.Writer.Written() {
				status = 500
			}
			if status >= 500 || fault != nil {
				level = slog.LevelError
			} else if status >= 400 {
				level = slog.LevelWarn
			}
			attrs := []any{"method", boundedMethod(c.Request.Method), "path", path, "status", status, "duration_ms", float64(time.Since(start).Microseconds()) / 1000}
			if fault != nil {
				attrs = append(attrs, "error", "handler panic")
			} else if code, ok := c.Get("gonex.error"); ok {
				attrs = append(attrs, "error", code)
			}
			requestLogger.Log(c.Request.Context(), level, "HTTP request", attrs...)
			if fault != nil {
				panic(fault)
			}
		}()
		c.Next()
	}
}

func safeKey(key string) string {
	for _, r := range key {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.') {
			return strconv.Quote(key)
		}
	}
	return key
}

func boundedMethod(method string) string {
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE", "CONNECT":
		return method
	default:
		return "OTHER"
	}
}
