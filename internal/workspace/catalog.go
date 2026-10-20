// Package workspace reads the local developer catalog. It is not service discovery.
package workspace

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const Filename = "gonex.yaml"

type Client struct {
	Address   string `yaml:"address" json:"address"`
	Transport string `yaml:"transport" json:"transport"`
}
type Service struct {
	Path    string            `yaml:"path" json:"path"`
	HTTP    string            `yaml:"http,omitempty" json:"http,omitempty"`
	GRPC    string            `yaml:"grpc,omitempty" json:"grpc,omitempty"`
	Clients map[string]Client `yaml:"clients,omitempty" json:"clients,omitempty"`
}
type Catalog struct {
	Schema   int                `yaml:"schema" json:"schema"`
	Services map[string]Service `yaml:"services" json:"services"`
}

var name = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func ValidName(s string) bool { return len(s) <= 64 && name.MatchString(s) }
func Parse(data []byte) (Catalog, error) {
	var c Catalog
	if len(data) > 1<<20 {
		return c, fmt.Errorf("gonex.yaml exceeds 1 MiB")
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(&c); err != nil {
		return c, fmt.Errorf("invalid gonex.yaml (schema, duplicate or unknown field); catalog must contain no secrets")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return c, fmt.Errorf("gonex.yaml must contain exactly one document")
	}
	return c, c.Validate()
}
func (c Catalog) Validate() error {
	if c.Schema != 1 || c.Services == nil || len(c.Services) > 256 {
		return fmt.Errorf("gonex.yaml requires schema: 1 and services")
	}
	ports := map[string]string{}
	paths := map[string]bool{}
	for _, n := range c.Names() {
		s := c.Services[n]
		if !ValidName(n) {
			return fmt.Errorf("invalid service name")
		}
		clean := filepath.Clean(s.Path)
		if clean != filepath.Join("services", n) || strings.ContainsAny(s.Path, "\\:\x00") || filepath.IsAbs(s.Path) {
			return fmt.Errorf("service %s path must be services/%s", n, n)
		}
		if paths[clean] {
			return fmt.Errorf("duplicate service path")
		}
		paths[clean] = true
		for _, a := range []string{s.HTTP, s.GRPC} {
			if a == "" {
				continue
			}
			_, port, err := Address(a)
			if err != nil {
				return fmt.Errorf("service %s requires a local loopback host:port", n)
			}
			if other := ports[port]; other != "" {
				return fmt.Errorf("duplicate listener port for %s and %s", other, n)
			}
			ports[port] = n
		}
		if len(s.Clients) > 256 {
			return fmt.Errorf("too many local client edges")
		}
		for target, cl := range s.Clients {
			if !ValidName(target) || cl.Transport != "grpc" {
				return fmt.Errorf("service %s has an invalid grpc client", n)
			}
			if _, _, err := Address(cl.Address); err != nil {
				return fmt.Errorf("service %s client %s requires a loopback host:port", n, target)
			}
		}
	}
	return nil
}
func Address(a string) (string, string, error) {
	h, p, e := net.SplitHostPort(a)
	if e != nil {
		return "", "", e
	}
	ip := net.ParseIP(h)
	if h != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", "", fmt.Errorf("local loopback required")
	}
	v, e := strconv.Atoi(p)
	if e != nil || v < 1 || v > 65535 {
		return "", "", fmt.Errorf("invalid port")
	}
	return h, strconv.Itoa(v), nil
}
func (c Catalog) Names() []string {
	names := make([]string, 0, len(c.Services))
	for n := range c.Services {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
func (c Catalog) Marshal() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return yaml.Marshal(c)
}
func Load(root string) (Catalog, []byte, error) {
	info, e := os.Lstat(filepath.Join(root, Filename))
	if e != nil {
		return Catalog{}, nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return Catalog{}, nil, fmt.Errorf("gonex.yaml must be a regular file of at most 1 MiB")
	}
	b, e := os.ReadFile(filepath.Join(root, Filename))
	if e != nil {
		return Catalog{}, nil, e
	}
	c, e := Parse(b)
	return c, b, e
}
func Find(dir string) (string, error) {
	root, e := filepath.Abs(dir)
	if e != nil {
		return "", e
	}
	for {
		if _, e := os.Stat(filepath.Join(root, Filename)); e == nil {
			return root, nil
		}
		p := filepath.Dir(root)
		if p == root {
			return "", fmt.Errorf("no gonex.yaml found; run from a microservices workspace or create its catalog")
		}
		root = p
	}
}

// CheckPaths rejects symlink traversal, including an existing workspace root.
func (c Catalog) CheckPaths(root string) error {
	for _, n := range c.Names() {
		p := filepath.Join(root, c.Services[n].Path)
		for cur := p; ; cur = filepath.Dir(cur) {
			i, e := os.Lstat(cur)
			if e != nil {
				return fmt.Errorf("service %s directory is missing", n)
			}
			if i.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("service %s uses a symlink path", n)
			}
			if cur == filepath.Dir(cur) {
				break
			}
		}
		if info, e := os.Lstat(filepath.Join(p, "go.mod")); e != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("service %s needs its own go.mod", n)
		}
	}
	return nil
}

// Cycles warns about declared edges; it does not infer runtime dependencies.
func (c Catalog) Cycles() []string {
	state := map[string]int{}
	var found []string
	var visit func(string)
	visit = func(n string) {
		if state[n] == 1 {
			found = append(found, n)
			return
		}
		if state[n] == 2 {
			return
		}
		state[n] = 1
		s := c.Services[n]
		targets := []string{}
		for t := range s.Clients {
			if _, ok := c.Services[t]; ok {
				targets = append(targets, t)
			}
		}
		sort.Strings(targets)
		for _, t := range targets {
			visit(t)
		}
		state[n] = 2
	}
	for _, n := range c.Names() {
		visit(n)
	}
	return found
}
