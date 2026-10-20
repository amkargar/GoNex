// Package terminal provides unobtrusive terminal styling.
package terminal

import (
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// Color enables ANSI only for a real terminal with color support.
func Color(out io.Writer) bool {
	f, ok := out.(*os.File)
	_, disabled := os.LookupEnv("NO_COLOR")
	return ok && !disabled && os.Getenv("TERM") != "dumb" && (isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd()))
}

func Paint(enabled bool, code, value string) string {
	if !enabled {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

// Safe keeps user-controlled labels on a single terminal line.
func Safe(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, value)
}
