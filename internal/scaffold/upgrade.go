package scaffold

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

// Upgrade updates dependency/tool metadata only; handwritten code and registries
// deliberately remain untouched. The caller reviews compatibility and runs tidy.
func Upgrade(root, frameworkVersion string) (*Plan, error) {
	if err := ValidFrameworkVersion(frameworkVersion); err != nil {
		return nil, fmt.Errorf("invalid framework version: %w", err)
	}
	m, oldManifest, err := Load(root)
	if err != nil {
		return nil, err
	}
	old, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	f, err := modfile.Parse("go.mod", old, nil)
	if err != nil {
		return nil, err
	}
	if err = f.AddRequire(Framework, frameworkVersion); err != nil {
		return nil, err
	}
	if err = f.AddTool(Framework + "/cmd/gonex"); err != nil {
		return nil, err
	}
	data, err := f.Format()
	if err != nil {
		return nil, err
	}
	p := &Plan{Root: root}
	if string(old) != string(data) {
		p.Changes = append(p.Changes, Change{Path: "go.mod", Data: data, Before: old, Update: true})
	}
	m.FrameworkVersion = frameworkVersion
	if err = p.manifest(m, oldManifest); err != nil {
		return nil, err
	}
	return p, nil
}
