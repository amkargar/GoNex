package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	base := t.TempDir()
	var out bytes.Buffer
	run := func(dir string, args ...string) int {
		out.Reset()
		return Execute(context.Background(), args, dir, &out, &out)
	}
	if code := run(base, "--help"); code != 0 || !strings.Contains(out.String(), "new NAME") {
		t.Fatal(code, out.String())
	}
	if code := run(base, "new", "shop", "--module", "example.com/shop", "--dry-run"); code != 0 {
		t.Fatal(out.String())
	}
	root := filepath.Join(base, "shop")
	if _, e := os.Stat(root); !os.IsNotExist(e) {
		t.Fatal("dry run mutated")
	}
	if code := run(base, "new", "shop", "--module", "example.com/shop"); code != 0 {
		t.Fatal(out.String())
	}
	if code := run(root, "create", "module", "users"); code != 0 {
		t.Fatal(out.String())
	}
	if code := run(root, "create", "module", "users"); code == 0 {
		t.Fatal("repeat succeeded")
	}
	if code := run(base, "create", "module", "users"); code == 0 || !strings.Contains(out.String(), "project root") {
		t.Fatal(out.String())
	}
	if code := run(root, "db", "migrate", "--dry-run"); code != 0 {
		t.Fatal(out.String())
	}
	if code := run(root, "openapi", "generate", "--dry-run"); code != 0 {
		t.Fatal(out.String())
	}
	if code := run(base, "unknown"); code == 0 {
		t.Fatal("unknown command succeeded")
	}
}
