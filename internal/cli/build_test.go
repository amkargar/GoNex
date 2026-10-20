package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/amkargar/gonex/internal/scaffold"
)

func TestBuildProducesRunnableExecutable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shop")
	plan, err := scaffold.New(root, "example.com/shop", "v0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if err = plan.Apply(false); err != nil {
		t.Fatal(err)
	}
	// A dependency-free fixture isolates the CLI's build/output behavior.
	for name, body := range map[string]string{
		"go.mod": "module example.com/shop\n\ngo 1.23.0\n",
		"main.go": "package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"built application runs\")}\n",
	} {
		if err = os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOOS", runtime.GOOS)
	t.Setenv("GOARCH", runtime.GOARCH)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	defaultFile := filepath.Join("dist", runtime.GOOS+"-"+runtime.GOARCH, "shop"+suffix)
	customFile := filepath.Join("release files", "custom"+suffix)
	for _, output := range []string{"", customFile} {
		var out, errout bytes.Buffer
		args := []string{"build", "--os", runtime.GOOS, "--arch", runtime.GOARCH}
		file := defaultFile
		if output != "" {
			args = append(args, "--out", output)
			file = output
		}
		if code := Execute(ctx, args, root, &out, &errout); code != 0 {
			t.Fatal(code, out.String(), errout.String())
		}
		if !strings.Contains(out.String(), "Build complete") || !strings.Contains(out.String(), file) || !strings.Contains(out.String(), "Run") {
			t.Fatal(out.String())
		}
		data, err := exec.CommandContext(ctx, filepath.Join(root, file)).CombinedOutput()
		if err != nil || !strings.Contains(string(data), "built application runs") {
			t.Fatal(err, string(data))
		}
	}
	var out, errout bytes.Buffer
	if Execute(ctx, []string{"build", "--os", "invalid", "--arch", "nope"}, root, &out, &errout) == 0 || strings.Contains(out.String(), "Build complete") {
		t.Fatal("unsupported target succeeded", out.String(), errout.String())
	}
	if err = os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nthis is invalid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errout.Reset()
	if Execute(ctx, []string{"build"}, root, &out, &errout) == 0 || strings.Contains(out.String(), "Build complete") || errout.Len() == 0 {
		t.Fatal("compiler failure was hidden", out.String(), errout.String())
	}
}
