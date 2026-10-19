package scaffold

import (
	"encoding/json"
	"fmt"
	transport "github.com/amkargar/gonex/http"
	"path/filepath"
	"strings"
)

// VersionNewModule scopes a not-yet-applied module plan without creating legacy URLs.
func VersionNewModule(p *Plan, name, version string) error {
	if !transport.ValidVersion(version) {
		return fmt.Errorf("invalid HTTP version")
	}
	manifestIndex := -1
	var m Manifest
	for i, change := range p.Changes {
		if change.Path == manifestFile {
			manifestIndex = i
			if err := json.Unmarshal(change.Data, &m); err != nil {
				return err
			}
		}
	}
	if manifestIndex < 0 {
		return fmt.Errorf("module plan has no manifest")
	}
	index := -1
	for i, entry := range m.Modules {
		if entry.Name == name {
			index = i
			break
		}
	}
	if index < 0 || !m.Modules[index].HasHTTP() {
		return fmt.Errorf("--http-version requires an HTTP module")
	}
	entry := &m.Modules[index]
	entry.Versions = []string{version}
	entry.Prefixes = map[string]string{version: "/api/" + version}
	entry.Replace = true
	registry, err := httpRegistry(m.Module, *entry)
	if err != nil {
		return err
	}
	registryPath := "modules/" + name + "/http_registry_gen.go"
	for i, change := range p.Changes {
		path := filepath.ToSlash(change.Path)
		prefix := "modules/" + name + "/http/"
		if strings.HasPrefix(path, prefix) {
			p.Changes[i].Path = filepath.FromSlash(prefix + version + "/" + strings.TrimPrefix(path, prefix))
		}
		if path == registryPath {
			p.Changes[i].Data = registry
		}
	}
	m.Owned[registryPath] = hash(registry)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	p.Changes[manifestIndex].Data = append(data, '\n')
	return nil
}
