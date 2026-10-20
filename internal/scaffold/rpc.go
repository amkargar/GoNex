package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/mod/modfile"
	"os"
	"path/filepath"
	"strings"
)

func dependency(old []byte, path string) ([]byte, error) {
	f, err := modfile.Parse("go.mod", old, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range f.Require {
		if r.Mod.Path == path {
			return old, nil
		}
	}
	if err = f.AddRequire(path, ContractsVersion); err != nil {
		return nil, err
	}
	return f.Format()
}
func title(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

func addRPC(p *Plan, m *Manifest, entry Module) error {
	if m.ContractsModule == "" {
		m.ContractsModule = m.Module + "/contracts"
		m.ContractsDir = "contracts"
		m.Service = filepath.Base(p.Root)
		if err := ValidName(m.Service); err != nil {
			return err
		}
		p.Changes = append(p.Changes, contractsFiles("contracts", m.ContractsModule)...)
		work := []byte("go 1.27.2\n\nuse (\n .\n ./contracts\n)\n")
		if err := p.owned(m, "go.work", work); err != nil {
			return err
		}
	}
	d := data{Framework: Framework, Project: m.Module, Name: entry.Name, Title: title(entry.Name), Provider: m.Service, Contracts: m.ContractsModule, RPCVersion: entry.RPCVersion()}
	for _, name := range []string{"server.go", "mapper.go"} {
		content, err := render("rpc_"+name, d)
		if err != nil {
			return err
		}
		p.Changes = append(p.Changes, Change{Path: filepath.Join("modules", entry.Name, "grpc", entry.RPCVersion(), name), Data: content})
	}
	content, err := render("module.proto", d)
	if err != nil {
		return err
	}
	p.Changes = append(p.Changes, Change{Path: filepath.Join(m.ContractsDir, "proto", m.Service, entry.RPCVersion(), entry.Name+".proto"), Data: content})
	// Local contracts are workspace members. Do not insert an unpublished
	// placeholder requirement; deployment selects a real published version explicitly.
	return nil
}

func EnableGRPC(root, name string) (*Plan, error) {
	if err := ValidName(name); err != nil {
		return nil, err
	}
	m, old, err := Load(root)
	if err != nil {
		return nil, err
	}
	index := -1
	for i, e := range m.Modules {
		if e.Name == name {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("module not found")
	}
	entry := &m.Modules[index]
	if entry.HasGRPC() {
		return nil, fmt.Errorf("gRPC already enabled; source preserved")
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "modules", name, "module.go"), nil, 0)
	if err != nil {
		return nil, err
	}
	factory := false
	injection := false
	ast.Inspect(file, func(node ast.Node) bool {
		if f, ok := node.(*ast.FuncDecl); ok && f.Name.Name == "NewModuleService" {
			factory = true
		}
		if field, ok := node.(*ast.Field); ok {
			for _, n := range field.Names {
				if n.Name == "Service" {
					injection = true
				}
			}
		}
		return true
	})
	if !factory || !injection {
		return nil, fmt.Errorf("module needs NewModuleService and injectable Module.Service; merge the current template factory manually, then retry; handwritten module.go preserved")
	}
	if entry.Transport == "none" {
		entry.Transport = "grpc"
	} else {
		entry.Transport = "http,grpc"
	}
	p := &Plan{Root: root}
	if err = addRPC(p, &m, *entry); err != nil {
		return nil, err
	}
	content, err := registry(m)
	if err != nil {
		return nil, err
	}
	if err = p.owned(&m, "modules/registry_gen.go", content); err != nil {
		return nil, err
	}
	if err = p.manifest(m, old); err != nil {
		return nil, err
	}
	return p, nil
}

func ContractsPath(root string) (string, error) {
	m, _, err := Load(root)
	if err != nil {
		return "", err
	}
	if m.ContractsDir != "" {
		return filepath.Clean(filepath.Join(root, m.ContractsDir)), nil
	}
	if _, err = os.Stat(filepath.Join(root, "contracts")); err == nil {
		return filepath.Join(root, "contracts"), nil
	}
	return "", fmt.Errorf("no contracts module configured")
}

func validRPCVersion(v string) bool {
	if len(v) < 2 || v[0] != 'v' || v[1] == '0' {
		return false
	}
	for _, r := range v[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
