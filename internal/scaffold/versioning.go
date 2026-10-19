package scaffold

import (
	"fmt"
	transport "github.com/amkargar/gonex/http"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

func VersionModule(root, name, version, policy, prefix string) (*Plan, error) {
	if err := ValidName(name); err != nil {
		return nil, err
	}
	if !transport.ValidVersion(version) {
		return nil, fmt.Errorf("invalid HTTP version")
	}
	if policy != "preserve" && policy != "replace" {
		return nil, fmt.Errorf("policy must be preserve or replace")
	}
	if prefix == "" {
		prefix = "/api/" + version
	}
	if _, err := transport.WithVersion(versionProbe{}, version, prefix); err != nil {
		return nil, err
	}
	m, old, err := Load(root)
	if err != nil {
		return nil, err
	}
	index := -1
	for i, entry := range m.Modules {
		if entry.Name == name {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("module %s does not exist", name)
	}
	entry := &m.Modules[index]
	if !entry.HasHTTP() {
		return nil, fmt.Errorf("module has no HTTP adapter")
	}
	for _, v := range entry.Versions {
		if v == version {
			return nil, fmt.Errorf("version %s already exists; handwritten files preserved", version)
		}
	}
	p := &Plan{Root: root}
	source := filepath.Join(root, "modules", name, "http")
	files, err := os.ReadDir(source)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}
		if file.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing symlink adapter source")
		}
		data, err := os.ReadFile(filepath.Join(source, file.Name()))
		if err != nil {
			return nil, err
		}
		if _, err = format.Source(data); err != nil {
			return nil, fmt.Errorf("invalid adapter source %s: %w", file.Name(), err)
		}
		p.Changes = append(p.Changes, Change{Path: filepath.Join("modules", name, "http", version, file.Name()), Data: data})
	}
	if len(p.Changes) == 0 {
		return nil, fmt.Errorf("no HTTP Go source files to version")
	}
	entry.Versions = append(entry.Versions, version)
	if entry.Prefixes == nil {
		entry.Prefixes = map[string]string{}
	}
	entry.Prefixes[version] = prefix
	if policy == "replace" {
		entry.Replace = true
	}
	content, err := httpRegistry(m.Module, *entry)
	if err != nil {
		return nil, err
	}
	if err = p.owned(&m, filepath.ToSlash(filepath.Join("modules", name, "http_registry_gen.go")), content); err != nil {
		return nil, err
	}
	if err = p.manifest(m, old); err != nil {
		return nil, err
	}
	return p, nil
}

type versionProbe struct{}

func (versionProbe) Endpoint(transport.Route) error { return nil }
