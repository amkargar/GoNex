package observability

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/amkargar/gonex/internal/terminal"
)

func (h *prettyHandler) levelLabel(level slog.Level) string {
	code := "36"
	if level >= slog.LevelError {
		code = "31"
	} else if level >= slog.LevelWarn {
		code = "33"
	}
	return terminal.Paint(h.color, code, fmt.Sprintf("%-5s", level.String()))
}

func (h *prettyHandler) startup(b *strings.Builder, attrs []slog.Attr) {
	values := map[string]string{}
	for _, a := range attrs {
		values[a.Key] = a.Value.Resolve().String()
	}
	name := values["name"]
	if name == "" {
		name = values["service"]
	}
	fmt.Fprintf(b, "\n%s  ·  %s\n\n  %s\n\n",
		terminal.Paint(h.color, "1;33", "GONEX"), terminal.Safe(name), terminal.Paint(h.color, "32", "Ready"))
	labels := map[string]string{"url": "Local", "listen": "Listening", "openapi": "OpenAPI", "docs": "Docs", "environment": "Environment", "debug": "Debug", "version": "Version"}
	printed := map[string]bool{}
	for _, key := range []string{"url", "listen", "openapi", "docs", "environment", "debug", "version"} {
		if value := values[key]; value != "" {
			fmt.Fprintf(b, "  %-12s %s\n", labels[key], terminal.Safe(value))
			printed[key] = true
		}
	}
	for _, a := range attrs {
		if strings.HasPrefix(a.Key, "openapi_") {
			fmt.Fprintf(b, "  %-12s %s\n", terminal.Safe("OpenAPI "+strings.TrimPrefix(a.Key, "openapi_")), terminal.Safe(a.Value.Resolve().String()))
			printed[a.Key] = true
		}
	}
	// Retain custom logger fields without hiding diagnostics.
	for _, a := range attrs {
		if !printed[a.Key] && a.Key != "name" && a.Key != "service" && a.Key != "stop" {
			b.WriteString("  ")
			writeAttr(b, "", a)
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n  Press Ctrl+C to stop.\n\n")
}

func (h *prettyHandler) request(b *strings.Builder, r slog.Record, attrs []slog.Attr) {
	values := map[string]slog.Value{}
	for _, a := range attrs {
		values[a.Key] = a.Value.Resolve()
	}
	status, _ := strconv.Atoi(values["status"].String())
	code := "32"
	if status >= 500 {
		code = "31"
	} else if status >= 400 {
		code = "33"
	}
	duration, err := strconv.ParseFloat(values["duration_ms"].String(), 64)
	if err != nil {
		duration = 0
	}
	fmt.Fprintf(b, "%s  %s  %-7s %-24s %s  %7.1f ms",
		r.Time.Format("15:04:05"), h.levelLabel(r.Level),
		terminal.Safe(values["method"].String()), terminal.Safe(values["path"].String()),
		terminal.Paint(h.color, code, values["status"].String()), duration)
	for _, a := range attrs {
		switch a.Key {
		case "method", "path", "status", "duration_ms":
		default:
			writeAttr(b, "", a)
		}
	}
	b.WriteByte('\n')
}
