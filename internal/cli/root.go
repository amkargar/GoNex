// Package cli implements the gonex command line.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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

const help = `gonex v0.1.0-dev
  new NAME --module IMPORT_PATH [--framework-version VERSION] [--dry-run]
  create module NAME [--transport http|none] [--db] [--dry-run]
  run | build | check
  openapi generate [--out openapi.json] [--dry-run]
  openapi validate [--file openapi.json]
  db migrate | db status [--dry-run]
Generators refuse handwritten overwrites. Run project commands from its root.
`

type invocation struct {
	ctx         context.Context
	dir         string
	out, errout io.Writer
}

func Execute(ctx context.Context, args []string, dir string, out, errout io.Writer) int {
	for _, arg := range args {
		if arg == "--help" {
			fmt.Fprint(out, help)
			return 0
		}
	}
	c := invocation{ctx: ctx, dir: dir, out: out, errout: errout}
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(out, help)
		return 0
	}
	if args[0] == "--version" {
		fmt.Fprintln(out, "v0.1.0-dev")
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
		version := f.String("framework-version", scaffold.Version, "framework module version")
		dry := f.Bool("dry-run", false, "show plan only")
		if err := f.Parse(args[2:]); err != nil {
			return err
		}
		if f.NArg() != 0 {
			return fmt.Errorf("unexpected arguments")
		}
		p, err := scaffold.New(filepath.Join(c.dir, args[1]), *module, *version)
		if err != nil {
			return err
		}
		return c.plan(p, *dry)
	case "create":
		if len(args) < 3 || args[1] != "module" {
			return fmt.Errorf("usage: gonex create module NAME")
		}
		f := c.flags("create module")
		transport := f.String("transport", "http", "http or none")
		db := f.Bool("db", false, "generate GORM persistence")
		dry := f.Bool("dry-run", false, "show plan only")
		if err := f.Parse(args[3:]); err != nil {
			return err
		}
		if f.NArg() != 0 {
			return fmt.Errorf("unexpected arguments")
		}
		p, err := scaffold.CreateModule(c.dir, args[2], *transport, *db)
		if err != nil {
			return err
		}
		return c.plan(p, *dry)
	case "run", "build", "check":
		if len(args) != 1 {
			return fmt.Errorf("unexpected arguments")
		}
		if _, _, err := scaffold.Load(c.dir); err != nil {
			return err
		}
		if args[0] == "run" {
			return c.goTool("run", ".")
		}
		if args[0] == "build" {
			return c.goTool("build", "./...")
		}
		for _, command := range []string{"test", "vet", "build"} {
			if err := c.goTool(command, "./..."); err != nil {
				return err
			}
		}
		return nil
	case "openapi":
		return c.openapi(args[1:])
	case "db":
		return c.database(args[1:])
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
	out := f.String("out", "openapi.json", "output artifact")
	file := f.String("file", "openapi.json", "document to validate")
	dry := f.Bool("dry-run", false, "show plan only")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if args[0] == "validate" {
		data, err := os.ReadFile(filepath.Join(c.dir, *file))
		if err != nil {
			return err
		}
		return openapi.Validate(data)
	}
	if args[0] != "generate" {
		return fmt.Errorf("unknown OpenAPI operation")
	}
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
	if err = c.goTool("run", ".", "--gonex-openapi", name); err != nil {
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
