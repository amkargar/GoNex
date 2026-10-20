package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/amkargar/gonex/internal/scaffold"
	"github.com/amkargar/gonex/internal/terminal"
)

func (c invocation) build(args []string) error {
	f := c.flags("build")
	targetOS := f.String("os", buildDefault("GOOS", runtime.GOOS), "target operating system")
	targetArch := f.String("arch", buildDefault("GOARCH", runtime.GOARCH), "target architecture")
	output := f.String("out", "", "output executable path")
	service := f.String("service", "", "workspace service")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments; run gonex build -h")
	}
	root, err := scaffold.ServiceRoot(c.dir, *service)
	if err != nil {
		return err
	}
	c.dir = root
	// Query Go rather than maintain a second, potentially stale target matrix.
	dist := exec.CommandContext(c.ctx, "go", "tool", "dist", "list")
	dist.Dir = root
	dist.Stderr = c.errout
	supported, err := dist.Output()
	if err != nil {
		return fmt.Errorf("query Go build targets: %w", err)
	}
	target := *targetOS + "/" + *targetArch
	valid := false
	for _, pair := range strings.Fields(string(supported)) {
		if pair == target {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unsupported target %q; use a GOOS/GOARCH pair from go tool dist list", target)
	}
	name := filepath.Base(root)
	if *output == "" {
		*output = filepath.Join("dist", *targetOS+"-"+*targetArch, name)
		if *targetOS == "windows" {
			*output += ".exe"
		}
	}
	destination := *output
	if !filepath.IsAbs(destination) {
		destination = filepath.Join(root, destination)
	}
	if info, statErr := os.Stat(destination); statErr == nil && info.IsDir() {
		return fmt.Errorf("--out must be an executable file path, not a directory")
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	color := terminal.Color(c.out)
	fmt.Fprintf(c.out, "\n%s  ·  Build\n\n  Building %s…\n\n",
		terminal.Paint(color, "1;33", "GONEX"), terminal.Safe(name))
	cmd := exec.CommandContext(c.ctx, "go", "build", "-o", destination, ".")
	cmd.Dir = root
	cmd.Env = mergeEnvironment(os.Environ(), []string{"GOOS=" + *targetOS, "GOARCH=" + *targetArch})
	cmd.Stdout = c.out
	cmd.Stderr = c.errout
	cmd.Stdin = os.Stdin
	// Preserve compiler diagnostics and the underlying Go command's exit status.
	if err = cmd.Run(); err != nil {
		return err
	}
	info, err := os.Stat(destination)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("go build did not produce an executable at %s", terminal.Safe(*output))
	}
	fmt.Fprintf(c.out, "  %s\n\n  Target   %s\n  Output   %s\n",
		terminal.Paint(color, "32", "Build complete"), target, terminal.Safe(*output))
	if *targetOS == runtime.GOOS && *targetArch == runtime.GOARCH {
		command := *output
		if !filepath.IsAbs(command) {
			command = "." + string(filepath.Separator) + command
		}
		command = terminal.Safe(command)
		if runtime.GOOS == "windows" {
			// PowerShell requires its call operator for a quoted executable path.
			command = "& '" + strings.ReplaceAll(command, "'", "''") + "'"
		} else if strings.ContainsAny(command, " \t'\"$\\`") {
			command = "'" + strings.ReplaceAll(command, "'", "'\"'\"'") + "'"
		}
		fmt.Fprintf(c.out, "\n  Run      %s\n", command)
	}
	fmt.Fprintln(c.out)
	return nil
}

func buildDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
