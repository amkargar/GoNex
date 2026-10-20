package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Service struct {
	Name        string `json:"name"`
	HTTPAddress string `json:"httpAddress"`
	GRPCAddress string `json:"grpcAddress"`
}

func NewMicroservices(root, modulePath, version string) (*Plan, error) {
	if err := ValidName(filepath.Base(root)); err != nil {
		return nil, err
	}
	if err := ValidModule(modulePath); err != nil {
		return nil, err
	}
	if err := ValidFrameworkVersion(version); err != nil {
		return nil, err
	}
	m := Manifest{Format: 1, Module: modulePath, FrameworkVersion: version, Architecture: "microservices", ContractsModule: modulePath + "/contracts", Modules: []Module{}, Services: []Service{}, Owned: map[string]string{}}
	p := &Plan{Root: root, New: true}
	toolmod, err := render("go.mod", data{Project: modulePath, Framework: Framework, Version: version})
	if err != nil {
		return nil, err
	}
	p.Changes = append(p.Changes, Change{Path: "go.mod", Data: toolmod})
	p.Changes = append(p.Changes, contractsFiles("contracts", m.ContractsModule)...)
	work := workspace(m)
	p.Changes = append(p.Changes, Change{Path: "go.work", Data: work})
	m.Owned["go.work"] = hash(work)
	p.Changes = append(p.Changes, Change{Path: "README.md", Data: []byte("# " + modulePath + "\n\nCreate services with gonex create service NAME. Create modules with --service NAME. Each service is an independent Go module. Run/check/build require --service when ambiguous.\n\nContracts live in contracts/. go.work is local development convenience; deployable modules need published dependencies. No production replace directives are generated.\n\nFor local plaintext development explicitly export GONEX_GRPC_INSECURE=1 and CLIENTS_IDENTITY_INSECURE=1. Production requires GRPC_TLS_CERT/KEY, optional GRPC_CLIENT_CA, and client trust/identity settings. No DB credentials are shared across services.\n")})
	if err := p.manifest(m, nil); err != nil {
		return nil, err
	}
	return p, nil
}
func workspace(m Manifest) []byte {
	var b strings.Builder
	b.WriteString("go 1.27.2\n\nuse (\n .\n ./contracts\n")
	for _, s := range m.Services {
		fmt.Fprintf(&b, " ./services/%s\n", s.Name)
	}
	b.WriteString(")\n")
	return []byte(b.String())
}
func contractsFiles(prefix, modulePath string) []Change {
	return []Change{
		{Path: filepath.Join(prefix, "go.mod"), Data: []byte("module " + modulePath + "\n\ngo 1.27.2\n\nrequire (\n google.golang.org/grpc v1.83.2\n google.golang.org/protobuf v1.36.11\n golang.org/x/net v0.60.0 // indirect\n golang.org/x/text v0.42.0 // indirect\n)\n")},
		{Path: filepath.Join(prefix, "buf.yaml"), Data: []byte("version: v2\nmodules:\n  - path: proto\nbreaking:\n  use: [FILE]\n")},
		{Path: filepath.Join(prefix, "buf.gen.yaml"), Data: []byte("version: v2\nplugins:\n  - local: protoc-gen-go\n    out: gen/go\n    opt: paths=source_relative\n  - local: protoc-gen-go-grpc\n    out: gen/go\n    opt: paths=source_relative\n")},
	}
}

func CreateService(root, name string) (*Plan, error) {
	if _, err := os.Lstat(filepath.Join(root, "services", name)); !os.IsNotExist(err) {
		return nil, fmt.Errorf("service destination already exists")
	}
	if err := ValidName(name); err != nil {
		return nil, err
	}
	m, old, err := Load(root)
	if err != nil {
		return nil, err
	}
	if m.Architecture != "microservices" {
		return nil, fmt.Errorf("create service requires a microservices workspace")
	}
	for _, s := range m.Services {
		if s.Name == name {
			return nil, fmt.Errorf("service already exists")
		}
	}
	service := Service{Name: name, HTTPAddress: fmt.Sprintf("127.0.0.1:%d", 8081+len(m.Services)), GRPCAddress: fmt.Sprintf("127.0.0.1:%d", 9091+len(m.Services))}
	child, err := New(filepath.Join(root, "services", name), m.Module+"/services/"+name, m.FrameworkVersion)
	if err != nil {
		return nil, err
	}
	var childManifest Manifest
	for i, c := range child.Changes {
		if c.Path == manifestFile {
			if err = json.Unmarshal(c.Data, &childManifest); err != nil {
				return nil, err
			}
			childManifest.Service = name
			childManifest.ContractsModule = m.ContractsModule
			childManifest.ContractsDir = "../../contracts"
			childManifest.GRPCAddress = service.GRPCAddress
			child.Changes[i].Data, err = json.MarshalIndent(childManifest, "", "  ")
			if err != nil {
				return nil, err
			}
		}
		if c.Path == "config/config.go" {
			child.Changes[i].Data, err = render("config.go", data{Framework: Framework, HTTPAddress: service.HTTPAddress})
			if err != nil {
				return nil, err
			}
		}
	}
	p := &Plan{Root: root}
	for _, c := range child.Changes {
		c.Path = filepath.Join("services", name, c.Path)
		p.Changes = append(p.Changes, c)
	}
	m.Services = append(m.Services, service)
	if err = p.owned(&m, "go.work", workspace(m)); err != nil {
		return nil, err
	}
	if err = p.manifest(m, old); err != nil {
		return nil, err
	}
	return p, nil
}

func ServiceRoot(root, name string) (string, error) {
	m, _, err := Load(root)
	if err != nil {
		return "", err
	}
	if m.Architecture != "microservices" {
		if name != "" {
			return "", fmt.Errorf("--service requires a workspace root")
		}
		return root, nil
	}
	if name == "" {
		if len(m.Services) == 1 {
			name = m.Services[0].Name
		} else {
			return "", fmt.Errorf("select --service NAME from the workspace root")
		}
	}
	if err = ValidName(name); err != nil {
		return "", err
	}
	for _, s := range m.Services {
		if s.Name == name {
			return filepath.Join(root, "services", name), nil
		}
	}
	return "", fmt.Errorf("unknown service %s", name)
}

// AddDependency updates only Go's dependency AST, preserving unrelated module settings.
func AddDependency(p *Plan, modulePath string) error {
	path := filepath.Join(p.Root, "go.mod")
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	data, err := dependency(old, modulePath)
	if err != nil {
		return err
	}
	if string(old) == string(data) {
		return nil
	}
	p.Changes = append(p.Changes, Change{Path: "go.mod", Data: data, Before: old, Update: true})
	return nil
}
