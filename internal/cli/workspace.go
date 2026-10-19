package cli

import (
	"fmt"
	"github.com/amkargar/gonex/internal/contracts"
	"github.com/amkargar/gonex/internal/scaffold"
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
	service := f.String("service", "", "workspace service")
	transport := f.String("transport", "http", "http, none, grpc or http,grpc")
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
	var p *scaffold.Plan
	var err error
	if args[0] == "service" {
		p, err = scaffold.CreateService(c.dir, args[1])
	} else {
		root, e := scaffold.ServiceRoot(c.dir, *service)
		if e != nil {
			return e
		}
		switch args[0] {
		case "module":
			p, err = scaffold.CreateModule(root, args[1], *transport, *db)
		case "client":
			p, err = c.clientPlan(root, args[1], *module, *version)
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
