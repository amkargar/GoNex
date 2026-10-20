// Public dependency/tutorial rehearsal. The contracts module is published to a
// disposable immutable file proxy; no replace directives or go.work are used.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	modmodule "golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "workspace rehearsal:", err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	target := os.Getenv("GONEX_SMOKE_VERSION")
	if target == "" {
		return fmt.Errorf("GONEX_SMOKE_VERSION must select a public commit/tag")
	}
	base, err := os.MkdirTemp("", "gonex-dx-public-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base)
	bin := filepath.Join(base, "bin")
	os.MkdirAll(bin, 0755)
	env := merge(os.Environ(), "GOWORK=off", "GOBIN="+bin, "GOTOOLCHAIN=local", "GOFLAGS=-p=2")
	command := func(dir string, environment []string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = environment
		cmd.WaitDelay = 10 * time.Second
		b, e := cmd.CombinedOutput()
		if e != nil {
			return b, fmt.Errorf("%s failed: %w\n%s", strings.Join(args, " "), e, b)
		}
		return b, nil
	}
	if _, err = command(base, env, "go", "install", "github.com/amkargar/gonex/cmd/gonex@"+target); err != nil {
		return err
	}
	cli := filepath.Join(bin, "gonex")
	if runtime.GOOS == "windows" {
		cli += ".exe"
	}
	invoke := func(dir string, args ...string) error {
		_, e := command(dir, env, append([]string{cli}, args...)...)
		return e
	}
	if err = invoke(base, "new", "commerce", "--module", "example.com/acme/commerce", "--arch", "microservices"); err != nil {
		return err
	}
	root := filepath.Join(base, "commerce")
	for _, args := range [][]string{{"create", "service", "identity", "--grpc"}, {"create", "service", "orders", "--http"}, {"create", "module", "users", "--service", "identity", "--transport", "grpc", "--version", "v1"}, {"create", "module", "orders", "--service", "orders", "--transport", "http"}, {"create", "client", "identity", "--service", "orders", "--module", "orders", "--version", "v1", "--profile", "example", "--rpc", "UsersService.GetUsersMessage"}, {"grpc", "generate", "--check"}} {
		if err = invoke(root, args...); err != nil {
			return err
		}
	}
	orders := filepath.Join(root, "services/orders")
	service := `package orders
import "context"
type IdentityReader interface{Message(context.Context)(string,error)}
type Service struct{identity IdentityReader}
func NewService(identity IdentityReader)*Service{return &Service{identity:identity}}
func(s *Service)Message(ctx context.Context)(string,error){return s.identity.Message(ctx)}
`
	if err = os.WriteFile(filepath.Join(orders, "modules/orders/service.go"), []byte(service), 0644); err != nil {
		return err
	}
	// The default generated test owns its original zero-dependency constructor;
	// adapt it deliberately to the caller's new plain-Go dependency.
	test := `package orders
import("context";"testing")
type fakeIdentity struct{}
func(fakeIdentity)Message(context.Context)(string,error){return "users",nil}
func TestBusiness(t *testing.T){got,err:=NewService(fakeIdentity{}).Message(context.Background());if got!="users"||err!=nil{t.Fatal(got,err)}}
`
	os.WriteFile(filepath.Join(orders, "modules/orders/service_test.go"), []byte(test), 0644)
	factory := filepath.Join(orders, "modules/orders/module.go")
	b, err := os.ReadFile(factory)
	if err != nil {
		return err
	}
	b = bytes.Replace(b, []byte("return NewService(), nil"), []byte("client, err := NewIdentityForModule(r); if err != nil { return nil, err }; return NewService(client), nil"), 1)
	if bytes.Contains(b, []byte("return NewService(), nil")) {
		return fmt.Errorf("tutorial factory edit did not apply")
	}
	os.WriteFile(factory, b, 0644)
	routes := filepath.Join(orders, "modules/orders/http/routes.go")
	b, _ = os.ReadFile(routes)
	b = bytes.Replace(b, []byte("Status: 200,"), []byte("Status: 200, Errors: []int{503},"), 1)
	os.WriteFile(routes, b, 0644)
	contracts := filepath.Join(root, "contracts")
	if _, err = command(contracts, env, "go", "mod", "tidy"); err != nil {
		return err
	}
	proxy := filepath.Join(base, "proxy")
	module := "example.com/acme/commerce/contracts"
	version := "v0.1.0"
	dir := filepath.Join(proxy, filepath.FromSlash(module), "@v")
	os.MkdirAll(dir, 0755)
	f, err := os.Create(filepath.Join(dir, version+".zip"))
	if err != nil {
		return err
	}
	err = modzip.CreateFromDir(f, modmodule.Version{Path: module, Version: version}, contracts)
	f.Close()
	if err != nil {
		return err
	}
	mod, _ := os.ReadFile(filepath.Join(contracts, "go.mod"))
	os.WriteFile(filepath.Join(dir, version+".mod"), mod, 0644)
	os.WriteFile(filepath.Join(dir, version+".info"), []byte(`{"Version":"v0.1.0","Time":"2026-10-20T00:00:00Z"}`), 0644)
	os.WriteFile(filepath.Join(dir, "list"), []byte(version+"\n"), 0644)
	archive, _ := os.ReadFile(filepath.Join(dir, version+".zip"))
	fmt.Printf("contracts publication fixture: %s@%s zip SHA256 %x\n", module, version, sha256.Sum256(archive))
	proxyURL := "file://" + filepath.ToSlash(proxy)
	if runtime.GOOS == "windows" {
		proxyURL = "file:///" + filepath.ToSlash(proxy)
	}
	env = merge(env, "GOPROXY="+proxyURL+",https://proxy.golang.org,direct", "GONOSUMDB="+module)
	for _, n := range []string{"identity", "orders"} {
		dir := filepath.Join(root, "services", n)
		mod, e := os.ReadFile(filepath.Join(dir, "go.mod"))
		if e != nil || bytes.Contains(mod, []byte("replace")) {
			return fmt.Errorf("service has replacement or missing module")
		}
		for _, args := range [][]string{{"mod", "tidy"}, {"test", "./..."}, {"vet", "./..."}, {"build", "./..."}} {
			if _, err = command(dir, env, append([]string{"go"}, args...)...); err != nil {
				return err
			}
		}
	}
	// Exercise every listener profile as an independently resolved module.
	for _, profile := range []struct {
		name  string
		flags []string
	}{{"mixed", []string{"--http", "--grpc"}}, {"worker", []string{"--transport", "none"}}} {
		if err = invoke(root, append([]string{"create", "service", profile.name}, profile.flags...)...); err != nil {
			return err
		}
		dir := filepath.Join(root, "services", profile.name)
		for _, args := range [][]string{{"mod", "tidy"}, {"build", "./..."}} {
			if _, err = command(dir, env, append([]string{"go"}, args...)...); err != nil {
				return err
			}
		}
	}
	rpcAddress, httpAddress := freeAddress(), freeAddress()
	env = merge(env, "GONEX_SERVICES_IDENTITY_GRPC_ADDRESS="+rpcAddress, "GONEX_SERVICES_ORDERS_HTTP_ADDRESS="+httpAddress)
	if err = invoke(root, "doctor", "--dev-insecure", "--json"); err != nil {
		return err
	}
	devCtx, stop := context.WithCancel(ctx)
	defer stop()
	cmd := exec.CommandContext(devCtx, cli, "dev", "--dev-insecure", "--isolated")
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.WaitDelay = 15 * time.Second
	// Let the CLI receive its own cancellation signal and reap owned binaries.
	cmd.Cancel = func() error {
		if runtime.GOOS == "windows" {
			return cmd.Process.Kill()
		}
		return cmd.Process.Signal(os.Interrupt)
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	client := http.Client{Timeout: 7 * time.Second}
	url := "http://" + httpAddress + "/orders"
	until := time.Now().Add(90 * time.Second)
	for {
		select {
		case e := <-done:
			return fmt.Errorf("dev exited before tutorial request: %v", e)
		default:
		}
		res, e := client.Get(url)
		if e == nil {
			body, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if res.StatusCode == 200 && bytes.Contains(body, []byte(`"message":"users"`)) {
				break
			}
		}
		if time.Now().After(until) {
			stop()
			<-done
			return fmt.Errorf("mapped tutorial request did not succeed")
		}
		time.Sleep(100 * time.Millisecond)
	}
	stop()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		return fmt.Errorf("dev did not exit")
	}
	for _, a := range []string{httpAddress, rpcAddress} {
		until = time.Now().Add(10 * time.Second)
		for {
			l, e := net.Listen("tcp", a)
			if e == nil {
				l.Close()
				break
			}
			if time.Now().After(until) {
				return fmt.Errorf("dev left an orphan listener")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	fmt.Println("PASS: public CLI, published contracts fixture, GOWORK=off, all transport profiles, doctor, mapped Orders -> Identity, supervised stop")
	return nil
}
func freeAddress() string {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		panic(e)
	}
	a := l.Addr().String()
	l.Close()
	return a
}
func merge(base []string, extra ...string) []string {
	m := map[string]string{}
	for _, v := range append(append([]string{}, base...), extra...) {
		k, value, _ := strings.Cut(v, "=")
		m[strings.ToUpper(k)] = value
	}
	out := []string{}
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}
