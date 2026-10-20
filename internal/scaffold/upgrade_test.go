package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradePreservesHandwrittenCode(t *testing.T) {
	root := project(t)
	p, err := CreateModule(root, "users", "http", false)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Apply(false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "modules/users/service.go")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := append(original, []byte("\n// Application-owned customization.\n")...)
	if err = os.WriteFile(path, edited, 0644); err != nil {
		t.Fatal(err)
	}
	plan, err := Upgrade(root, "v0.1.0-rc.1")
	if err != nil {
		t.Fatal(err)
	}
	if err = plan.Apply(true); err != nil {
		t.Fatal(err)
	}
	m, _, _ := Load(root)
	if m.FrameworkVersion == "v0.1.0-rc.1" {
		t.Fatal("dry run mutated manifest")
	}
	if err = plan.Apply(false); err != nil {
		t.Fatal(err)
	}
	m, _, _ = Load(root)
	if m.FrameworkVersion != "v0.1.0-rc.1" {
		t.Fatal(m.FrameworkVersion)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(edited) {
		t.Fatal("handwritten code changed")
	}
	mod, _ := os.ReadFile(filepath.Join(root, "go.mod"))
	if !strings.Contains(string(mod), Framework+" v0.1.0-rc.1") || !strings.Contains(string(mod), "tool "+Framework+"/cmd/gonex") {
		t.Fatal(string(mod))
	}
	if _, err = Upgrade(root, "not-a-version"); err == nil {
		t.Fatal("invalid version accepted")
	}
}
