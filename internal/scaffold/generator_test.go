package scaffold

import (
	"bytes"
	"context"
	"encoding/json"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func project(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "shop")
	p, e := New(root, "example.com/acme/shop", Version)
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	return root
}
func TestSafeGeneration(t *testing.T) {
	root := project(t)
	p, e := CreateModule(root, "users", "http", false)
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Apply(true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(root, "modules", "users")); !os.IsNotExist(e) {
		t.Fatal("dry run wrote files")
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	handler := filepath.Join(root, "modules", "users", "http", "handlers.go")
	edited := []byte("package adapter\n// handwritten customization\n")
	if e = os.WriteFile(handler, edited, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = CreateModule(root, "users", "http", false); e == nil {
		t.Fatal("repeat did not fail")
	}
	got, _ := os.ReadFile(handler)
	if !bytes.Equal(got, edited) {
		t.Fatal("handwritten file changed")
	}
	if _, e = CreateModule(root, "../escape", "http", false); e == nil {
		t.Fatal("invalid name accepted")
	}
	if _, e = CreateModule(root, "type", "none", false); e == nil {
		t.Fatal("keyword accepted")
	}
	p, e = CreateModule(root, "reports", "none", false)
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(root, "modules", "reports", "http")); !os.IsNotExist(e) {
		t.Fatal("core-only module has HTTP directory")
	}
	registry := filepath.Join(root, "modules", "registry_gen.go")
	data, _ := os.ReadFile(registry)
	if strings.Count(string(data), "users.Module{}") != 1 || strings.Count(string(data), "reports.Module{}") != 1 {
		t.Fatal(string(data))
	}
	formatted, e := format.Source(data)
	if e != nil || !bytes.Equal(data, formatted) {
		t.Fatal("registry not formatted", e)
	}
	if e = os.WriteFile(registry, append(data, []byte("// edited\n")...), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = CreateModule(root, "other", "none", false); e == nil {
		t.Fatal("edited registry accepted")
	}
	if _, e = os.Stat(filepath.Join(root, "modules", "other")); !os.IsNotExist(e) {
		t.Fatal("conflict caused partial output")
	}
}

func TestProjectPlanDeterministic(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shop")
	a, e := New(root, "example.com/shop", Version)
	if e != nil {
		t.Fatal(e)
	}
	b, e := New(root, "example.com/shop", Version)
	if e != nil {
		t.Fatal(e)
	}
	for i, c := range a.Changes {
		if c.Path != b.Changes[i].Path || !bytes.Equal(c.Data, b.Changes[i].Data) {
			t.Fatal("nondeterministic generation")
		}
	}
	if e = a.Apply(true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(root); !os.IsNotExist(e) {
		t.Fatal("dry run created project")
	}
	if e = a.Apply(false); e != nil {
		t.Fatal(e)
	}
	if e = b.Apply(false); e == nil {
		t.Fatal("existing project overwritten")
	}
}

func TestGeneratedProjectBuilds(t *testing.T) {
	root := project(t)
	for _, m := range []Module{{Name: "users", Transport: "http"}, {Name: "reports", Transport: "none"}, {Name: "messages", Transport: "http", DB: true}, {Name: "storage", Transport: "none", DB: true}} {
		p, e := CreateModule(root, m.Name, m.Transport, m.DB)
		if e != nil {
			t.Fatal(e)
		}
		if e = p.Apply(false); e != nil {
			t.Fatal(e)
		}
	}
	framework, e := filepath.Abs(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// This isolated harness override never enters generated templates.
	for _, args := range [][]string{{"mod", "edit", "-replace=" + Framework + "=" + filepath.ToSlash(framework)}, {"mod", "tidy"}, {"test", "./..."}, {"vet", "./..."}, {"build", "./..."}, {"run", ".", "--gonex-openapi", "openapi.json"}} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOWORK=off", "DATABASE_DIALECT=sqlite", "DATABASE_URL=file:"+filepath.ToSlash(filepath.Join(root, "fixture.db")))
		output, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("go %v: %v\n%s", args, e, output)
		}
	}
}

func TestPlanRejectsTraversalAndChangedFiles(t *testing.T) {
	root := project(t)
	for _, path := range []string{"../escape.go", filepath.Join(t.TempDir(), "absolute.go")} {
		p := Plan{Root: root, Changes: []Change{{Path: path, Data: []byte("bad")}}}
		if e := p.Apply(false); e == nil {
			t.Fatal("unsafe path accepted")
		}
	}
	path := filepath.Join(root, "main.go")
	old, _ := os.ReadFile(path)
	p := Plan{Root: root, Changes: []Change{{Path: "main.go", Before: old, Data: []byte("changed"), Update: true}}}
	if e := os.WriteFile(path, []byte("new edit"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := p.Apply(false); e == nil {
		t.Fatal("stale plan accepted")
	}
}

func TestTemplateGolden(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shop")
	p, e := New(root, "example.com/acme/shop", Version)
	if e != nil {
		t.Fatal(e)
	}
	files := map[string]string{}
	for _, c := range p.Changes {
		files[filepath.ToSlash(c.Path)] = string(c.Data)
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	p, e = CreateModule(root, "users", "http", false)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range p.Changes {
		files[filepath.ToSlash(c.Path)] = string(c.Data)
	}
	data, e := json.MarshalIndent(files, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join("testdata", "project.golden.json")
	if os.Getenv("GONEX_UPDATE_GOLDEN") == "1" {
		if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	want, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(data, want) {
		t.Fatal("template output changed; review golden before explicitly updating")
	}
}
