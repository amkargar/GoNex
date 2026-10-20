package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextualHelpHasNoProjectSideEffects(t *testing.T) {
	root := t.TempDir()
	for topic, entry := range helpTopics {
		for _, flag := range []string{"-h", "--help"} {
			t.Run(topic+"/"+flag, func(t *testing.T) {
				var out, errout bytes.Buffer
				args := append(strings.Fields(topic), flag)
				if code := Execute(context.Background(), args, root, &out, &errout); code != 0 {
					t.Fatal(code, out.String(), errout.String())
				}
				if !strings.Contains(out.String(), entry.usage) || errout.Len() != 0 {
					t.Fatal(out.String(), errout.String())
				}
				if strings.Contains(out.String(), "\x1b") {
					t.Fatal("redirected help contains ANSI escapes")
				}
			})
		}
	}
	for _, args := range [][]string{nil, {"-h"}, {"--help"}, {"help"}, {"help", "create", "module"}} {
		var out, errout bytes.Buffer
		if Execute(context.Background(), args, root, &out, &errout) != 0 || errout.Len() != 0 {
			t.Fatal(args, out.String(), errout.String())
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("help mutated the working directory", entries, err)
	}
	var out, errout bytes.Buffer
	if Execute(context.Background(), []string{"create", "unknown", "-h"}, filepath.Join(root, "missing"), &out, &errout) == 0 {
		t.Fatal("unknown command help succeeded")
	}
}
