package scaffold

import (
	"encoding/json"
	"fmt"
	catalog "github.com/amkargar/gonex/internal/workspace"
	"net"
	"os"
	"path/filepath"
	"strconv"
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
	p.Changes = append(p.Changes, Change{Path: catalog.Filename, Data: []byte("schema: 1\nservices: {}\n")})
	work := workspace(m)
	p.Changes = append(p.Changes, Change{Path: "go.work", Data: work})
	m.Owned["go.work"] = hash(work)
	p.Changes = append(p.Changes, Change{Path: "README.md", Data: []byte("# " + modulePath + "\n\nPrepare root dependencies explicitly with go mod tidy. Create services with gonex create service NAME --http or --grpc (both flags for mixed; --transport none for a worker). Create modules with --service NAME. Complete client mapping/wiring, then use gonex doctor and gonex dev. gonex.yaml is caller-editable local configuration; it contains no secrets. Each service is an independent Go module. Run/check/build require --service when ambiguous.\n\nContracts live in contracts/. go.work is local development convenience; deployable modules need published dependencies. No production replace directives are generated.\n\nFor local plaintext development explicitly export GONEX_GRPC_INSECURE=1 and CLIENTS_IDENTITY_INSECURE=1. Production requires GRPC_TLS_CERT/KEY, optional GRPC_CLIENT_CA, and client trust/identity settings. No DB credentials are shared across services.\n")})
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
	return CreateServiceWithTransports(root, name, true, false)
}

func CreateServiceWithTransports(root, name string, httpEnabled, grpcEnabled bool) (*Plan, error) {
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
	cat, catBefore, err := catalog.Load(root)
	if os.IsNotExist(err) {
		cat = catalog.Catalog{Schema: 1, Services: map[string]catalog.Service{}}
		for _, s := range m.Services {
			cat.Services[s.Name] = catalog.Service{Path: "services/" + s.Name, HTTP: s.HTTPAddress}
		}
	} else if err != nil {
		return nil, err
	}
	used := map[int]bool{}
	for _, s := range cat.Services {
		for _, a := range []string{s.HTTP, s.GRPC} {
			_, port, _ := net.SplitHostPort(a)
			v, _ := strconv.Atoi(port)
			used[v] = true
		}
	}
	next := func(start int) (string, error) {
		for v := start; v <= 65535; v++ {
			if !used[v] {
				used[v] = true
				return fmt.Sprintf("127.0.0.1:%d", v), nil
			}
		}
		return "", fmt.Errorf("no local ports available")
	}
	httpAddress, err := next(8081)
	if err != nil {
		return nil, err
	}
	grpcAddress, err := next(9091)
	if err != nil {
		return nil, err
	}
	service := Service{Name: name, HTTPAddress: httpAddress, GRPCAddress: grpcAddress}
	local := catalog.Service{Path: "services/" + name}
	if httpEnabled {
		local.HTTP = httpAddress
	}
	if grpcEnabled {
		local.GRPC = grpcAddress
	}
	cat.Services[name] = local
	catalogData, err := cat.Marshal()
	if err != nil {
		return nil, err
	}
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
			childManifest.GRPCEnabled = grpcEnabled
			if grpcEnabled {
				childManifest.Owned["modules/registry_gen.go"] = ""
			}
			child.Changes[i].Data, err = json.MarshalIndent(childManifest, "", "  ")
			if err != nil {
				return nil, err
			}
		}
		if c.Path == "config/config.go" {
			child.Changes[i].Data, err = render("config.go", data{Framework: Framework, HTTPAddress: service.HTTPAddress, DisableHTTP: !httpEnabled})
			if err != nil {
				return nil, err
			}
		}
	}
	if grpcEnabled {
		registryData, e := registry(childManifest)
		if e != nil {
			return nil, e
		}
		childManifest.Owned["modules/registry_gen.go"] = hash(registryData)
		for i := range child.Changes {
			if child.Changes[i].Path == "modules/registry_gen.go" {
				child.Changes[i].Data = registryData
			}
			if child.Changes[i].Path == manifestFile {
				child.Changes[i].Data, _ = json.MarshalIndent(childManifest, "", "  ")
			}
		}
	}
	p := &Plan{Root: root}
	p.Changes = append(p.Changes, Change{Path: catalog.Filename, Data: catalogData, Before: catBefore, Update: catBefore != nil})
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
