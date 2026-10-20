// Package cli implements the gonex command line.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/internal/buildinfo"
	"github.com/amkargar/gonex/internal/scaffold"
	"github.com/amkargar/gonex/migration"
	"github.com/amkargar/gonex/openapi"
	"github.com/amkargar/gonex/orm"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Version may be set by release builds with -ldflags. Module build metadata takes precedence.
var Version = "development"

func version() string {
	v := buildinfo.Version()
	if v == "development" {
		return Version
	}
	return v
}

type invocation struct {
	ctx         context.Context
	dir         string
	out, errout io.Writer
}

func Execute(ctx context.Context, args []string, dir string, out, errout io.Writer) int {
	c := invocation{ctx: ctx, dir: dir, out: out, errout: errout}
	topic, requested, helpErr := helpRequest(args)
	if requested {
		if helpErr != nil {
			fmt.Fprintln(errout, "gonex:", helpErr)
			return 1
		}
		c.showHelp(topic)
		return 0
	}
	if args[0] == "--version" {
		fmt.Fprintln(out, version())
		return 0
	}
	err := c.execute(args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err == nil {
		return 0
	}
	fmt.Fprintln(errout, "gonex:", err)
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 1
}
func (c invocation) flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(c.errout)
	return f
}
func (c invocation) plan(p *scaffold.Plan, dry bool) error {
	if err := p.Apply(true); err != nil {
		return err
	}
	for _, change := range p.Changes {
		action := "create"
		if change.Update {
			action = "update"
		}
		fmt.Fprintf(c.out, "%s %s\n", action, change.Path)
	}
	if dry {
		fmt.Fprintln(c.out, "dry run: no files changed")
		return nil
	}
	return p.Apply(false)
}
func (c invocation) execute(args []string) error {
	switch args[0] {
	case "dev", "doctor":
		return c.development(args)
	case "new":
		if len(args) > 1 {
			if err := scaffold.ValidName(args[1]); err != nil {
				return err
			}
		}
		if len(args) < 2 {
			return fmt.Errorf("usage: gonex new NAME --module IMPORT_PATH")
		}
		f := c.flags("new")
		module := f.String("module", "", "Go module path")
		version := f.String("framework-version", "", "framework module version (defaults to the installed CLI version)")
		arch := f.String("arch", "monolith", "monolith or microservices")
		dry := f.Bool("dry-run", false, "show plan only")
		if err := f.Parse(args[2:]); err != nil {
			return err
		}
		if f.NArg() != 0 {
			return fmt.Errorf("unexpected arguments")
		}
		if *version == "" {
			*version = buildinfo.Version()
			if *version == "development" {
				*version = Version
			}
		}
		if scaffold.ValidFrameworkVersion(*version) != nil {
			return fmt.Errorf("cannot determine a resolvable Gonex version; install a versioned CLI or pass --framework-version with a Go module version")
		}
		p, err := scaffold.New(filepath.Join(c.dir, args[1]), *module, *version)
		if *arch == "microservices" {
			p, err = scaffold.NewMicroservices(filepath.Join(c.dir, args[1]), *module, *version)
		} else if *arch != "monolith" {
			return fmt.Errorf("unknown architecture %s", *arch)
		}
		if err != nil {
			return err
		}
		return c.plan(p, *dry)
	case "upgrade":
		f := c.flags("upgrade")
		version := f.String("framework-version", "", "exact target Go module version")
		dry := f.Bool("dry-run", false, "show metadata changes only")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if f.NArg() != 0 || *version == "" {
			return fmt.Errorf("usage: gonex upgrade --framework-version VERSION [--dry-run]")
		}
		plan, err := scaffold.Upgrade(c.dir, *version)
		if err != nil {
			return err
		}
		if err = c.plan(plan, *dry); err != nil {
			return err
		}
		fmt.Fprintln(c.out, "Review compatibility, run go mod tidy and gonex check; handwritten code and SQL migrations were not migrated.")
		return nil
	case "create":
		return c.create(args[1:])
	case "build":
		return c.build(args[1:])
	case "run", "check":
		f := c.flags(args[0])
		service := f.String("service", "", "workspace service")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if f.NArg() != 0 {
			return fmt.Errorf("unexpected arguments")
		}
		root, err := scaffold.ServiceRoot(c.dir, *service)
		if err != nil {
			return err
		}
		c.dir = root
		if args[0] == "run" {
			err := c.goTool("run", ".")
			// The development command's context is cancelled by Ctrl+C/SIGTERM.
			// go run can report an exit error even after the application drains.
			if errors.Is(c.ctx.Err(), context.Canceled) {
				return nil
			}
			return err
		}
		for _, command := range []string{"test", "vet", "build"} {
			if err := c.goTool(command, "./..."); err != nil {
				return err
			}
		}
		return nil
	case "openapi":
		return c.openapi(args[1:])
	case "module":
		if len(args) >= 3 && args[1] == "enable" {
			f := c.flags("module enable")
			enabled := f.Bool("grpc", false, "enable gRPC")
			service := f.String("service", "", "workspace service")
			dry := f.Bool("dry-run", false, "show plan only")
			if err := f.Parse(args[3:]); err != nil {
				return err
			}
			if !*enabled || f.NArg() != 0 {
				return fmt.Errorf("usage: gonex module enable NAME --grpc")
			}
			root, err := scaffold.ServiceRoot(c.dir, *service)
			if err != nil {
				return err
			}
			p, err := scaffold.EnableGRPC(root, args[2])
			if err != nil {
				return err
			}
			return c.completePlan(p, *dry)
		}
		if len(args) < 3 || args[1] != "version" {
			return fmt.Errorf("usage: gonex module version NAME --http v1 [--policy preserve|replace]")
		}
		f := c.flags("module version")
		service := f.String("service", "", "workspace service")
		version := f.String("http", "", "HTTP version")
		policy := f.String("policy", "preserve", "legacy route policy")
		prefix := f.String("prefix", "", "version URL prefix")
		dry := f.Bool("dry-run", false, "show plan only")
		if err := f.Parse(args[3:]); err != nil {
			return err
		}
		if f.NArg() != 0 {
			return fmt.Errorf("unexpected arguments")
		}
		root, err := scaffold.ServiceRoot(c.dir, *service)
		if err != nil {
			return err
		}
		p, err := scaffold.VersionModule(root, args[2], *version, *policy, *prefix)
		if err != nil {
			return err
		}
		return c.completePlan(p, *dry)
	case "db":
		return c.database(args[1:])
	case "grpc":
		return c.grpc(args[1:])
	default:
		return fmt.Errorf("unknown command %q; run gonex --help", args[0])
	}
}
func (c invocation) goTool(args ...string) error {
	cmd := exec.CommandContext(c.ctx, "go", args...)
	cmd.Dir = c.dir
	cmd.Stdout = c.out
	cmd.Stderr = c.errout
	cmd.Stdin = os.Stdin
	cmd.WaitDelay = 10 * time.Second
	cmd.Cancel = func() error {
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	return cmd.Run()
}
func (c invocation) openapi(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gonex openapi generate|validate")
	}
	f := c.flags("openapi")
	service := f.String("service", "", "workspace service")
	out := f.String("out", "openapi.json", "output artifact")
	file := f.String("file", "openapi.json", "document to validate")
	against := f.String("against", "", "compatibility baseline document")
	version := f.String("http-version", "", "HTTP document version")
	dry := f.Bool("dry-run", false, "show plan only")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if args[0] == "validate" {
		if *service != "" {
			root, err := scaffold.ServiceRoot(c.dir, *service)
			if err != nil {
				return err
			}
			c.dir = root
		}
		data, err := os.ReadFile(filepath.Join(c.dir, *file))
		if err != nil {
			return err
		}
		if err = openapi.Validate(data); err != nil {
			return err
		}
		if *against != "" {
			baseline, err := os.ReadFile(filepath.Join(c.dir, *against))
			if err != nil {
				return err
			}
			return openapi.Compatible(baseline, data)
		}
		return nil
	}
	if args[0] != "generate" {
		return fmt.Errorf("unknown OpenAPI operation")
	}
	root, err := scaffold.ServiceRoot(c.dir, *service)
	if err != nil {
		return err
	}
	c.dir = root
	if _, _, err := scaffold.Load(c.dir); err != nil {
		return err
	}
	if filepath.Ext(*out) != ".json" || filepath.IsAbs(*out) || strings.Contains(*out, "..") {
		return fmt.Errorf("output must be a relative JSON artifact path")
	}
	if *dry {
		fmt.Fprintln(c.out, "generate", *out, "(dry run: no files changed)")
		return nil
	}
	temp, err := os.CreateTemp(c.dir, ".gonex-openapi-*.json")
	if err != nil {
		return err
	}
	name := temp.Name()
	temp.Close()
	defer os.Remove(name)
	arguments := []string{"run", ".", "--gonex-openapi", name}
	if *version != "" {
		if !transport.ValidVersion(*version) {
			return fmt.Errorf("invalid HTTP version")
		}
		arguments = append(arguments, *version)
	}
	if err = c.goTool(arguments...); err != nil {
		return err
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if err = openapi.Validate(data); err != nil {
		return err
	}
	before, readErr := os.ReadFile(filepath.Join(c.dir, *out))
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	p := &scaffold.Plan{Root: c.dir, Changes: []scaffold.Change{{Path: *out, Data: data, Before: before, Update: readErr == nil}}}
	return c.plan(p, false)
}
func (c invocation) database(args []string) error {
	if len(args) == 0 || args[0] != "migrate" && args[0] != "status" {
		return fmt.Errorf("usage: gonex db migrate|status")
	}
	if _, _, err := scaffold.Load(c.dir); err != nil {
		return err
	}
	f := c.flags("db")
	service := f.String("service", "", "workspace service")
	dry := f.Bool("dry-run", false, "show operation without connecting")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if *dry {
		fmt.Fprintln(c.out, "db", args[0], "(dry run: no connection or changes)")
		return nil
	}
	root, err := scaffold.ServiceRoot(c.dir, *service)
	if err != nil {
		return err
	}
	c.dir = root
	dialect := os.Getenv("DATABASE_DIALECT")
	if dialect == "" {
		dialect = "sqlite"
	}
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()
	db, err := orm.Open(ctx, orm.Options{Dialect: dialect, DSNEnv: "DATABASE_URL"})
	if err != nil {
		return err
	}
	defer db.Close(context.Background())
	pool, err := db.SQL()
	if err != nil {
		return err
	}
	runner, err := migration.New(pool, dialect, os.DirFS(filepath.Join(c.dir, "migrations")))
	if err != nil {
		return err
	}
	if args[0] == "migrate" {
		return runner.Up(ctx)
	}
	status, err := runner.Status(ctx)
	if err != nil {
		return err
	}
	return json.NewEncoder(c.out).Encode(status)
}
