package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	goversion "go/version"
	"google.golang.org/protobuf/proto"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	rpc "github.com/amkargar/gonex/grpc"
	"github.com/amkargar/gonex/internal/contracts"
	"github.com/amkargar/gonex/internal/devrunner"
	"github.com/amkargar/gonex/internal/workspace"
)

func (c invocation) development(args []string) error {
	f := c.flags(args[0])
	service := f.String("service", "", "select one service (otherwise all)")
	insecure := f.Bool("dev-insecure", false, "explicit loopback development plaintext; forbidden in production")
	jsonOutput := f.Bool("json", false, "doctor diagnostics as JSON")
	generate := f.Bool("generate", false, "dev only: explicitly run pinned contract generation before startup")
	isolated := f.Bool("isolated", false, "dev only: build with GOWORK=off; contracts must be published/resolvable")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if args[0] == "doctor" && (*generate || *isolated) {
		return fmt.Errorf("--generate/--isolated apply to dev")
	}
	if args[0] == "dev" && *jsonOutput {
		return fmt.Errorf("--json applies to doctor; dev streams prefixed service output")
	}
	root, err := workspace.Find(c.dir)
	if err != nil {
		return err
	}
	cat, _, err := workspace.Load(root)
	if err != nil {
		return err
	}
	p, err := workspace.Resolve(cat, root, *service, *insecure, os.Getenv)
	if err != nil {
		return err
	}
	// Parse local TLS material without echoing filenames, tokens or raw errors.
	for _, name := range p.Names {
		s := p.Catalog.Services[name]
		if s.GRPC != "" && !(*insecure || os.Getenv("GONEX_GRPC_INSECURE") == "1") {
			if _, e := rpc.ServerTLS(os.Getenv("GRPC_TLS_CERT"), os.Getenv("GRPC_TLS_KEY"), os.Getenv("GRPC_CLIENT_CA")); e != nil {
				p.Issues = append(p.Issues, workspace.Issue{Severity: "error", Code: "invalid_server_tls", Service: name, Message: "server TLS material cannot be loaded; verify certificate, key and optional client CA"})
			}
		}
		for target := range s.Clients {
			prefix := "CLIENTS_" + strings.ToUpper(target) + "_"
			if *insecure || os.Getenv(prefix+"INSECURE") == "1" {
				continue
			}
			if _, e := rpc.ClientTLS(os.Getenv(prefix+"CA"), os.Getenv(prefix+"SERVER_NAME"), os.Getenv(prefix+"CERT"), os.Getenv(prefix+"KEY")); e != nil {
				p.Issues = append(p.Issues, workspace.Issue{Severity: "error", Code: "invalid_client_tls", Service: name, Message: "client TLS material cannot be loaded; verify CA, server name and optional certificate/key"})
			}
		}
	}
	if _, err = exec.LookPath("go"); err != nil {
		p.Issues = append(p.Issues, workspace.Issue{Severity: "error", Code: "go_missing", Message: "install supported Go and add it to PATH"})
	}
	contractRoot := filepath.Join(root, "contracts")
	if *generate && !p.HasErrors() {
		if _, e := os.Stat(filepath.Join(contractRoot, "proto")); e == nil {
			child := c
			child.dir = root
			if err = child.grpc([]string{"generate"}); err != nil {
				return err
			}
		}
	}
	toolCtx, toolCancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer toolCancel()
	tool := exec.CommandContext(toolCtx, "go", "version")
	tool.Env = mergeEnvironment(os.Environ(), []string{"GOTOOLCHAIN=local"})
	toolVersion, toolErr := tool.Output()
	parts := strings.Fields(string(toolVersion))
	if toolErr != nil || len(parts) < 3 || goversion.Compare(parts[2], "go1.27.2") < 0 {
		p.Issues = append(p.Issues, workspace.Issue{Severity: "error", Code: "go_version", Message: "PATH must select Go 1.27.2 or newer; dev will not download a toolchain"})
	}

	if _, e := os.Stat(filepath.Join(contractRoot, "proto")); e == nil {
		set, _, compileErr := contracts.Compile(c.ctx, contractRoot)
		if compileErr != nil {
			p.Issues = append(p.Issues, workspace.Issue{Severity: "error", Code: "contracts_invalid", Path: "contracts/proto", Message: "contract compilation failed; run gonex grpc generate --dry-run for descriptor diagnostics"})
		}
		if schema, e := os.ReadFile(filepath.Join(contractRoot, "schema.binpb")); e == nil && compileErr == nil {
			current, _ := proto.MarshalOptions{Deterministic: true}.Marshal(set)
			if !bytes.Equal(schema, current) {
				p.Issues = append(p.Issues, workspace.Issue{Severity: "error", Code: "contracts_stale", Path: "contracts", Message: "schema differs from proto source; explicitly run gonex grpc generate"})
			}
		} else if e != nil {
			p.Issues = append(p.Issues, workspace.Issue{Severity: "error", Code: "contracts_not_generated", Path: "contracts", Message: "generated schema is missing; explicitly run gonex grpc generate"})
		}
	}
	if args[0] == "doctor" && *jsonOutput {
		err = json.NewEncoder(c.out).Encode(struct {
			Schema   int               `json:"schema"`
			CLI      string            `json:"cli"`
			Runtime  string            `json:"runtime"`
			Services []string          `json:"services"`
			Issues   []workspace.Issue `json:"issues"`
		}{1, version(), runtime.Version(), p.Names, p.Issues})
		if err != nil {
			return err
		}
	} else {
		fmt.Fprintf(c.out, "Gonex local workspace · CLI %s · %s\n", version(), runtime.Version())
		for _, n := range p.Names {
			s := p.Catalog.Services[n]
			fmt.Fprintf(c.out, "  %s  HTTP=%s  gRPC=%s\n", n, display(s.HTTP), display(s.GRPC))
		}
		for _, i := range p.Issues {
			fmt.Fprintf(c.out, "%s [%s] %s %s: %s\n", i.Severity, i.Code, i.Service, i.Path, i.Message)
		}
	}
	if p.HasErrors() {
		return fmt.Errorf("workspace diagnostics failed; resolve the reported configuration/mapping issues")
	}
	if args[0] == "doctor" {
		return nil
	}
	temp, err := os.MkdirTemp("", "gonex-dev-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	commands := make([]devrunner.Command, 0, len(p.Names))
	for _, n := range p.Names {
		dir := filepath.Join(root, p.Catalog.Services[n].Path)
		fmt.Fprintf(c.out, "[%s] building (cached dependencies; no module edits or tool downloads)\n", n)
		binary, e := devrunner.Build(c.ctx, dir, temp, n, *isolated, c.out)
		if e != nil {
			return e
		}
		s := p.Catalog.Services[n]
		commands = append(commands, devrunner.Command{Name: n, Dir: dir, HTTP: s.HTTP, GRPC: s.GRPC, Args: []string{binary}, Env: mergeEnvironment(os.Environ(), p.Environment[n])})
	}
	fmt.Fprintln(c.out, "Starting local services. Ctrl+C stops all owned processes. Listener URLs are configured addresses; application logs/readiness confirm availability.")
	err = devrunner.Supervise(c.ctx, devrunner.Options{Commands: commands, Output: c.out, Grace: 10 * time.Second})
	if c.ctx.Err() != nil {
		return nil
	}
	return err
}
func display(s string) string {
	if s == "" {
		return "disabled"
	}
	return s
}
func mergeEnvironment(base, extra []string) []string {
	values := map[string]string{}
	for _, v := range extra {
		k, value, _ := strings.Cut(v, "=")
		values[strings.ToUpper(k)] = value
	}
	result := []string{}
	for _, v := range base {
		k, _, _ := strings.Cut(v, "=")
		if _, ok := values[strings.ToUpper(k)]; !ok {
			result = append(result, v)
		}
	}
	keys := []string{}
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result = append(result, k+"="+values[k])
	}
	return result
}
