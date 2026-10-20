package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/amkargar/gonex/internal/contracts"
	"github.com/amkargar/gonex/internal/scaffold"
	"github.com/amkargar/gonex/internal/workspace"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func (c invocation) create(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: gonex create module|service|client NAME")
	}
	f := c.flags("create " + args[0])
	httpEnabled := f.Bool("http", false, "enable HTTP listener for a service")
	grpcEnabled := f.Bool("grpc", false, "enable gRPC listener for a service")
	profile := f.String("profile", "skeleton", "client profile: skeleton, example or raw")
	rpcMethod := f.String("rpc", "", "explicit Service.Method for the example profile")
	service := f.String("service", "", "workspace service")
	transport := f.String("transport", "http", "http, none, grpc or http,grpc")
	httpVersion := f.String("http-version", "", "create an initially versioned HTTP adapter")
	db := f.Bool("db", false, "generate persistence")
	dry := f.Bool("dry-run", false, "show plan only")
	module := f.String("module", "", "consumer module for client adapter")
	version := f.String("version", "v1", "contract major version")
	if err := f.Parse(args[2:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	explicit := map[string]bool{}
	f.Visit(func(v *flag.Flag) { explicit[v.Name] = true })
	if args[0] != "service" && (explicit["http"] || explicit["grpc"]) {
		return fmt.Errorf("--http/--grpc select service listeners; use --transport for a module")
	}
	allowed := map[string]bool{"dry-run": true}
	switch args[0] {
	case "service":
		for _, v := range []string{"http", "grpc", "transport"} {
			allowed[v] = true
		}
	case "module":
		for _, v := range []string{"service", "transport", "db", "http-version", "version"} {
			allowed[v] = true
		}
	case "client":
		for _, v := range []string{"service", "module", "version", "profile", "rpc"} {
			allowed[v] = true
		}
	}
	for k := range explicit {
		if !allowed[k] {
			return fmt.Errorf("--%s does not apply to create %s", k, args[0])
		}
	}
	var p *scaffold.Plan
	var err error
	if args[0] == "service" {
		if *service != "" || *db || explicit["module"] || explicit["version"] || explicit["http-version"] || explicit["profile"] || explicit["rpc"] {
			return fmt.Errorf("service accepts --http, --grpc or --transport none|http|grpc|http,grpc")
		}
		h, g := true, false
		if explicit["http"] || explicit["grpc"] {
			h, g = *httpEnabled, *grpcEnabled
		}
		if explicit["transport"] {
			if explicit["http"] || explicit["grpc"] {
				return fmt.Errorf("choose service --transport or --http/--grpc, not both")
			}
			switch *transport {
			case "none":
				h, g = false, false
			case "http":
				h, g = true, false
			case "grpc":
				h, g = false, true
			case "http,grpc":
				h, g = true, true
			default:
				return fmt.Errorf("invalid service transport")
			}
		}
		p, err = scaffold.CreateServiceWithTransports(c.dir, args[1], h, g)
	} else {
		root, e := scaffold.ServiceRoot(c.dir, *service)
		if e != nil {
			return e
		}
		switch args[0] {
		case "module":
			p, err = scaffold.CreateModuleVersion(root, args[1], *transport, *db, *version)
			if err == nil && *httpVersion != "" {
				err = scaffold.VersionNewModule(p, args[1], *httpVersion)
			}
		case "client":
			p, err = c.clientProfilePlan(root, args[1], *module, *version, *profile, *rpcMethod)
		default:
			return fmt.Errorf("unknown create target")
		}
	}
	if err != nil {
		return err
	}
	return c.completePlan(p, *dry)
}

// Rebase service edits into their owning workspace before validation; all
// final paths must stay inside that workspace, including shared contracts.
func rebaseWorkspace(p *scaffold.Plan) error {
	m, _, err := scaffold.Load(p.Root)
	if err != nil {
		return err
	}
	if m.ContractsDir != "../../contracts" {
		return nil
	}
	workspace := filepath.Clean(filepath.Join(p.Root, "..", ".."))
	owner, _, err := scaffold.Load(workspace)
	if err != nil || owner.Architecture != "microservices" {
		return fmt.Errorf("shared contracts require a valid owning workspace")
	}
	expected, err := scaffold.ServiceRoot(workspace, m.Service)
	if err != nil {
		return err
	}
	left, _ := filepath.Abs(expected)
	right, _ := filepath.Abs(p.Root)
	if left != right {
		return fmt.Errorf("service is outside its declared workspace")
	}
	for i := range p.Changes {
		absolute := filepath.Join(p.Root, p.Changes[i].Path)
		p.Changes[i].Path, err = filepath.Rel(workspace, absolute)
		if err != nil {
			return err
		}
	}
	p.Root = workspace
	return nil
}
func (c invocation) completePlan(p *scaffold.Plan, dry bool) error {
	if !p.New {
		if err := rebaseWorkspace(p); err != nil {
			return err
		}
	}
	if err := syncCatalog(p); err != nil {
		return err
	}
	if err := p.Apply(true); err != nil {
		return err
	}
	contractRoots := map[string]bool{}
	for _, change := range p.Changes {
		path := filepath.ToSlash(change.Path)
		if strings.HasSuffix(path, ".proto") {
			if index := strings.Index(path, "/proto/"); index >= 0 {
				contractRoots[path[:index]] = true
			}
		}
	}
	if dry || len(contractRoots) == 0 {
		return c.plan(p, dry)
	}
	for prefix := range contractRoots {
		stage, err := os.MkdirTemp("", "gonex-contracts-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(stage)
		source := filepath.Join(p.Root, prefix)
		err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) && path == source {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink contracts are not supported")
			}
			rel, e := filepath.Rel(source, path)
			if e != nil {
				return e
			}
			target := filepath.Join(stage, rel)
			if entry.IsDir() {
				return os.MkdirAll(target, 0755)
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			return os.WriteFile(target, data, 0644)
		})
		if err != nil {
			return err
		}
		for _, change := range p.Changes {
			path := filepath.ToSlash(change.Path)
			if !strings.HasPrefix(path, prefix+"/") {
				continue
			}
			target := filepath.Join(stage, strings.TrimPrefix(path, prefix+"/"))
			if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err = os.WriteFile(target, change.Data, 0644); err != nil {
				return err
			}
		}
		generated, err := contracts.Generate(c.ctx, stage, false)
		if err != nil {
			return err
		}
		for _, change := range generated.Changes {
			change.Path = filepath.Join(prefix, change.Path)
			p.Changes = append(p.Changes, change)
		}
	}
	return c.plan(p, false)
}

// Catalog is caller-editable local configuration; generator updates use an exact
// before-image, not ownership hashes, and never touch production credentials.
func syncCatalog(p *scaffold.Plan) error {
	owner, _, err := scaffold.Load(p.Root)
	if err != nil || owner.Architecture != "microservices" {
		return nil
	}
	for _, ch := range p.Changes {
		if filepath.ToSlash(ch.Path) == workspace.Filename {
			return nil
		}
	}
	cat, before, err := workspace.Load(p.Root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	changed := false
	for _, ch := range p.Changes {
		if !strings.HasSuffix(filepath.ToSlash(ch.Path), "/.gonex.json") {
			continue
		}
		var m scaffold.Manifest
		if err = json.Unmarshal(ch.Data, &m); err != nil {
			return err
		}
		s, ok := cat.Services[m.Service]
		if !ok {
			continue
		}
		for _, mod := range m.Modules {
			if mod.HasGRPC() && s.GRPC == "" {
				s.GRPC = m.GRPCAddress
				changed = true
			}
		}
		cat.Services[m.Service] = s
	}
	if changed {
		b, e := cat.Marshal()
		if e != nil {
			return e
		}
		p.Changes = append(p.Changes, scaffold.Change{Path: workspace.Filename, Data: b, Before: before, Update: true})
	}
	return nil
}
