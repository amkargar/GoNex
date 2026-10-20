package workspace

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Issue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Service  string `json:"service,omitempty"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}
type Prepared struct {
	Catalog     Catalog
	Names       []string
	Environment map[string][]string
	Issues      []Issue
}

// Resolve honors service-scoped overrides; shared listener overrides are rejected
// for multiple services to prevent accidentally binding them to the same port.
func Resolve(c Catalog, root, selected string, insecure bool, lookup func(string) string) (Prepared, error) {
	p := Prepared{Catalog: c, Environment: map[string][]string{}}
	// Do not mutate the parsed caller-owned maps.
	p.Catalog.Services = map[string]Service{}
	for n, s := range c.Services {
		copy := s
		copy.Clients = map[string]Client{}
		for k, v := range s.Clients {
			copy.Clients[k] = v
		}
		p.Catalog.Services[n] = copy
	}
	if selected != "" {
		if _, ok := c.Services[selected]; !ok {
			return p, fmt.Errorf("unknown selected service")
		}
		p.Names = []string{selected}
	} else {
		p.Names = c.Names()
	}
	if len(p.Names) == 0 {
		return p, fmt.Errorf("workspace has no services; create service NAME first")
	}
	if len(p.Names) > 1 && (lookup("HTTP_ADDRESS") != "" || lookup("GRPC_ADDRESS") != "") {
		return p, fmt.Errorf("shared HTTP_ADDRESS/GRPC_ADDRESS is ambiguous; use GONEX_SERVICES_NAME_HTTP_ADDRESS/GRPC_ADDRESS")
	}
	if insecure && strings.EqualFold(strings.TrimSpace(lookup("GONEX_ENV")), "production") {
		return p, fmt.Errorf("--dev-insecure is forbidden with GONEX_ENV=production")
	}
	for _, n := range c.Names() {
		s := p.Catalog.Services[n]
		prefix := "GONEX_SERVICES_" + strings.ToUpper(n) + "_"
		for _, v := range []struct {
			key string
			dst *string
		}{{"HTTP_ADDRESS", &s.HTTP}, {"GRPC_ADDRESS", &s.GRPC}} {
			if address := lookup(prefix + v.key); address != "" {
				if *v.dst == "" {
					return p, fmt.Errorf("service %s has no %s listener to override", n, v.key)
				}
				*v.dst = address
			} else if len(p.Names) == 1 && p.Names[0] == n && *v.dst != "" && lookup(v.key) != "" {
				*v.dst = lookup(v.key)
			}
		}
		p.Catalog.Services[n] = s
	}
	if err := p.Catalog.Validate(); err != nil {
		return p, err
	}
	if err := p.Catalog.CheckPaths(root); err != nil {
		return p, err
	}
	issue := func(severity, code, n, path, msg string) {
		p.Issues = append(p.Issues, Issue{severity, code, n, path, msg})
	}
	for _, n := range p.Names {
		s := p.Catalog.Services[n]
		env := []string{"APP_NAME=" + n, "GONEX_ENV=development"}
		if value := lookup("GONEX_ENV"); value != "" {
			env[1] = "GONEX_ENV=" + value
		}
		if s.HTTP != "" {
			env = append(env, "HTTP_ADDRESS="+s.HTTP, "GONEX_DISABLE_HTTP=0")
		} else {
			env = append(env, "GONEX_DISABLE_HTTP=1")
		}
		if s.GRPC != "" {
			env = append(env, "GRPC_ADDRESS="+s.GRPC)
			allow := insecure || lookup("GONEX_GRPC_INSECURE") == "1"
			if strings.EqualFold(strings.TrimSpace(lookup("GONEX_ENV")), "production") && allow {
				issue("error", "production_plaintext", n, "", "development insecure flags forbidden in production")
			}
			if allow {
				if lookup("GRPC_TLS_CERT") != "" || lookup("GRPC_TLS_KEY") != "" || lookup("GRPC_CLIENT_CA") != "" {
					issue("error", "security_flags_conflict", n, "", "unset development insecure flags when server TLS material is configured")
				}
				env = append(env, "GONEX_GRPC_INSECURE=1")
				issue("warning", "dev_plaintext", n, "", "explicit local gRPC plaintext enabled")
			} else if lookup("GRPC_TLS_CERT") == "" || lookup("GRPC_TLS_KEY") == "" {
				issue("error", "server_tls_missing", n, "", "set GRPC_TLS_CERT/GRPC_TLS_KEY or explicitly pass --dev-insecure for local development")
			}
		}
		targets := []string{}
		for t := range s.Clients {
			targets = append(targets, t)
		}
		sort.Strings(targets)
		for _, target := range targets {
			cl := s.Clients[target]
			prefix := "CLIENTS_" + strings.ToUpper(target) + "_"
			address := lookup(prefix + "ADDRESS")
			if address == "" {
				address = cl.Address
				if provider, ok := p.Catalog.Services[target]; ok {
					address = provider.GRPC
				}
			}
			if _, _, e := Address(address); e != nil {
				return p, fmt.Errorf("service %s client %s must use a local loopback target", n, target)
			}
			env = append(env, prefix+"ADDRESS="+address)
			allow := insecure || lookup(prefix+"INSECURE") == "1"
			if strings.EqualFold(strings.TrimSpace(lookup("GONEX_ENV")), "production") && allow {
				issue("error", "production_plaintext", n, "", "development insecure client flags forbidden in production")
			}
			if allow {
				if lookup(prefix+"CA") != "" || lookup(prefix+"CERT") != "" || lookup(prefix+"KEY") != "" {
					issue("error", "security_flags_conflict", n, "", "unset development insecure flags when client TLS material is configured")
				}
				env = append(env, prefix+"INSECURE=1")
				issue("warning", "dev_plaintext", n, "", "explicit local client plaintext enabled")
			} else if lookup(prefix+"CA") == "" {
				issue("error", "client_tls_missing", n, "", "configure the provider client CA or explicitly pass --dev-insecure locally")
			}
		}
		for _, a := range []string{s.HTTP, s.GRPC} {
			if a == "" {
				continue
			}
			l, e := net.Listen("tcp", a)
			if e != nil {
				issue("error", "port_unavailable", n, "", "declared listener port is unavailable")
			} else {
				l.Close()
			}
		}
		serviceRoot := filepath.Join(root, s.Path)
		helpers := map[string]string{}
		calls := map[string]bool{}
		err := filepath.WalkDir(serviceRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("service %s contains a symlink; inspect the workspace", n)
			}
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if info.Size() > 1<<20 {
				return fmt.Errorf("service %s source file exceeds diagnostic limit", n)
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(root, path)
			tree, parseErr := parser.ParseFile(token.NewFileSet(), path, b, 0)
			if parseErr != nil {
				return nil
			}
			if strings.Contains(string(b), "GONEX_WIRING_REQUIRED") {
				for _, d := range tree.Decls {
					if f, ok := d.(*ast.FuncDecl); ok && strings.HasPrefix(f.Name.Name, "New") && strings.HasSuffix(f.Name.Name, "ForModule") {
						helpers[f.Name.Name] = filepath.ToSlash(rel)
					}
				}
			}
			ast.Inspect(tree, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					switch fn := call.Fun.(type) {
					case *ast.Ident:
						calls[fn.Name] = true
					case *ast.SelectorExpr:
						calls[fn.Sel.Name] = true
					}
				}
				return true
			})

			if strings.Contains(string(b), "GONEX_TODO_") {
				issue("error", "mapping_todo", n, filepath.ToSlash(rel), "implement explicit contract-to-domain mapping and wiring; see docs/microservices-dx/quickstart.md")
			}
			return nil
		})
		if err != nil {
			return p, err
		}
		for helper, path := range helpers {
			if !calls[helper] {
				issue("error", "wiring_missing", n, path, "call "+helper+" once from the module factory and inject it into the plain-Go service; see the quickstart")
			}
		}
		p.Environment[n] = env
	}
	for _, n := range p.Catalog.Cycles() {
		issue("warning", "dependency_cycle", n, "", "declared catalog has a cycle; startup order is not a dependency guarantee")
	}
	return p, nil
}
func (p Prepared) HasErrors() bool {
	for _, i := range p.Issues {
		if i.Severity == "error" {
			return true
		}
	}
	return false
}
