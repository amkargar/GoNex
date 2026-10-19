package scaffold

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/token"
	"golang.org/x/mod/module"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const Framework = "github.com/amkargar/gonex"
const Version = "v0.1.0"
const manifestFile = ".gonex.json"

type Module struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	DB        bool     `json:"db,omitempty"`
	Versions  []string `json:"versions,omitempty"`
	Replace   bool     `json:"replace,omitempty"`
}
type Manifest struct {
	Format           int               `json:"format"`
	Module           string            `json:"module"`
	FrameworkVersion string            `json:"frameworkVersion"`
	Modules          []Module          `json:"modules"`
	Owned            map[string]string `json:"owned"`
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func ValidName(name string) error {
	if !namePattern.MatchString(name) || token.Lookup(name).IsKeyword() || name == "internal" || name == "vendor" || name == "main" {
		return fmt.Errorf("%q must be a lowercase non-keyword Go identifier", name)
	}
	base := strings.ToUpper(strings.Split(name, ".")[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || regexp.MustCompile(`^(COM|LPT)[0-9]$`).MatchString(base) {
		return fmt.Errorf("reserved filesystem name %q", name)
	}
	return nil
}
func ValidModule(path string) error { return module.CheckPath(path) }

func ValidFrameworkVersion(version string) error { return module.Check(Framework, version) }
func hash(data []byte) string                    { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func Load(root string) (Manifest, []byte, error) {
	data, err := os.ReadFile(filepath.Join(root, manifestFile))
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("run from a Gonex project root containing .gonex.json")
	}
	var m Manifest
	if err = json.Unmarshal(data, &m); err != nil {
		return m, nil, fmt.Errorf("invalid .gonex.json: %w", err)
	}
	if m.Format != 1 || ValidModule(m.Module) != nil || m.Owned == nil {
		return m, nil, fmt.Errorf("unsupported or invalid project manifest")
	}
	for _, entry := range m.Modules {
		if err = ValidName(entry.Name); err != nil {
			return m, nil, err
		}
	}
	return m, data, nil
}
func (p *Plan) owned(m *Manifest, path string, data []byte) error {
	old, err := os.ReadFile(filepath.Join(p.Root, path))
	update := err == nil
	if update && m.Owned[path] != hash(old) {
		return fmt.Errorf("handwritten changes in %s; merge them before generation", path)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	p.Changes = append(p.Changes, Change{Path: path, Data: data, Before: old, Update: update})
	m.Owned[path] = hash(data)
	return nil
}
func (p *Plan) manifest(m Manifest, old []byte) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	p.Changes = append(p.Changes, Change{Path: manifestFile, Data: data, Before: old, Update: old != nil})
	return nil
}
