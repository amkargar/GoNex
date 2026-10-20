package cli

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMicroservicesGeneratedEndToEnd(t *testing.T) {
	t.Setenv("GOFLAGS", "-p=2")
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	base := t.TempDir()
	root := filepath.Join(base, "commerce")
	var out bytes.Buffer
	run := func(dir string, args ...string) {
		t.Helper()
		t.Logf("gonex %v", args)
		out.Reset()
		if code := Execute(ctx, args, dir, &out, &out); code != 0 {
			t.Fatalf("gonex %v (%d): %s", args, code, &out)
		}
	}
	run(base, "new", "commerce", "--module", "example.com/acme/commerce", "--arch", "microservices", "--framework-version", "v0.0.0")
	run(root, "create", "service", "identity")
	run(root, "create", "service", "orders")
	run(root, "create", "module", "users", "--service", "identity", "--transport", "grpc", "--dry-run")
	if _, err := os.Stat(filepath.Join(root, "contracts/proto")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote contracts")
	}
	run(root, "create", "module", "users", "--service", "identity", "--transport", "grpc")
	run(root, "create", "module", "audit", "--service", "identity", "--transport", "http")
	auditPath := filepath.Join(root, "services/identity/modules/audit/module.go")
	auditBefore, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	run(root, "module", "enable", "audit", "--service", "identity", "--grpc")
	auditAfter, err := os.ReadFile(auditPath)
	if err != nil || !bytes.Equal(auditBefore, auditAfter) {
		t.Fatal("enable changed handwritten module")
	}
	run(root, "create", "module", "orders", "--service", "orders", "--transport", "http")
	run(root, "create", "client", "identity", "--service", "orders", "--module", "orders", "--version", "v1", "--profile", "raw")
	if _, err := os.Stat(filepath.Join(root, "services/identity/modules/users/http")); !os.IsNotExist(err) {
		t.Fatal("gRPC-only module has HTTP adapter")
	}
	if _, err := os.Stat(filepath.Join(root, "contracts/gen/go/identity/v1/users_grpc.pb.go")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := Execute(ctx, []string{"build"}, root, &out, &out); code == 0 || !strings.Contains(out.String(), "--service") {
		t.Fatal("ambiguous service not rejected", out.String())
	}
	framework, _ := filepath.Abs("../..")
	contractRoot := filepath.Join(root, "contracts")
	goCmd := func(dir string, args ...string) {
		t.Helper()
		t.Logf("go %v in %s", args, filepath.Base(dir))
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go %v: %v\n%s", args, err, output)
		}
	}
	goCmd(contractRoot, "mod", "tidy")
	goCmd(contractRoot, "test", "./...")
	for _, name := range []string{"identity", "orders"} {
		dir := filepath.Join(root, "services", name)
		initial, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(initial), "replace") {
			t.Fatal("production template contains a replace directive")
		}
		goCmd(dir, "mod", "edit", "-replace=github.com/amkargar/gonex="+filepath.ToSlash(framework), "-replace=example.com/acme/commerce/contracts="+filepath.ToSlash(contractRoot))
		goCmd(dir, "mod", "tidy")
		goCmd(dir, "test", "./...")
		goCmd(dir, "build", "./...")
	}
	// Developer-filled domain mapping: generation deliberately does not guess it.
	ordersDir := filepath.Join(root, "services/orders")
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ordersDir, path), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("modules/orders/identity_mapping.go", `package orders
import("context";pb "example.com/acme/commerce/contracts/gen/go/identity/v1";rpc "github.com/amkargar/gonex/grpc")
func(c *IdentityClient)Message(ctx context.Context)(string,error){response,err:=c.UsersService.GetUsersMessage(ctx,&pb.GetUsersMessageRequest{});if err!=nil{return "",rpc.FromStatus(err)};return response.Message,nil}
`)
	write("modules/orders/service.go", `package orders
import "context"
type MessageReader interface{Message(context.Context)(string,error)}
type Service struct{Reader MessageReader}
func NewService()*Service{return &Service{}}
func(s *Service)Message(ctx context.Context)(string,error){if s.Reader!=nil{return s.Reader.Message(ctx)};return "orders",ctx.Err()}
`)
	write("modules/orders/module.go", `package orders
import("github.com/amkargar/gonex/app";rpc "github.com/amkargar/gonex/grpc";"example.com/acme/commerce/services/orders/config")
type Module struct{Service *Service}
func(m Module)Register(r *app.Registrar)error{s:=m.Service;if s==nil{var err error;s,err=NewModuleService(r);if err!=nil{return err}};return registerHTTP(r,s)}
func NewModuleService(r *app.Registrar)(*Service,error){settings,err:=config.LoadIdentityClient();if err!=nil{return nil,err};pool:=rpc.NewClients();if err=r.Manage(pool);err!=nil{return nil,err};client,err:=NewIdentityClient(pool,settings.Options());if err!=nil{return nil,err};service:=NewService();service.Reader=client;return service,nil}
`)
	routesPath := filepath.Join(ordersDir, "modules/orders/http/routes.go")
	routes, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatal(err)
	}
	needle := []byte("Tags(\"orders\"),")
	if !bytes.Contains(routes, needle) {
		t.Fatal("generated route declaration changed; update the failure contract explicitly")
	}
	routes = bytes.ReplaceAll(routes, needle, []byte("Tags(\"orders\").Errors(503),"))
	if err = os.WriteFile(routesPath, routes, 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"identity", "orders"} {
		dir := filepath.Join(root, "services", name)
		goCmd(dir, "test", "./...")
		goCmd(dir, "build", "-o", filepath.Join(dir, name+".exe"), ".")
	}
	rpcAddress := freeAddress(t)
	httpAddress := freeAddress(t)
	identity := launch(t, ctx, filepath.Join(root, "services/identity/identity.exe"), []string{"GRPC_ADDRESS=" + rpcAddress, "GONEX_GRPC_INSECURE=1", "GONEX_DISABLE_HTTP=1"})
	launch(t, ctx, filepath.Join(ordersDir, "orders.exe"), []string{"HTTP_ADDRESS=" + httpAddress, "CLIENTS_IDENTITY_ADDRESS=" + rpcAddress, "CLIENTS_IDENTITY_INSECURE=1"})
	client := http.Client{Timeout: 6 * time.Second}
	url := "http://" + httpAddress + "/orders"
	deadline := time.Now().Add(10 * time.Second)
	for {
		response, err := client.Get(url)
		if err == nil {
			data, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode == 200 && strings.Contains(string(data), `"message":"users"`) {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("generated services did not communicate")
		}
		time.Sleep(20 * time.Millisecond)
	}
	identity.stop()
	started := time.Now()
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 503 || time.Since(started) > 5500*time.Millisecond {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("provider failure: %d %s", response.StatusCode, data)
	}

	response.Body.Close()
	launch(t, ctx, filepath.Join(root, "services/identity/identity.exe"), []string{"GRPC_ADDRESS=" + rpcAddress, "GONEX_GRPC_INSECURE=1", "GONEX_DISABLE_HTTP=1"})
	recoveryDeadline := time.Now().Add(25 * time.Second)
	for {
		res, e := client.Get(url)
		if e == nil {
			data, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if res.StatusCode == 200 && strings.Contains(string(data), `"message":"users"`) {
				break
			}
		}
		if time.Now().After(recoveryDeadline) {
			t.Fatal("consumer did not recover its existing pooled connection")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

type process struct {
	command *exec.Cmd
	done    chan struct{}
}

func (p *process) stop() {
	select {
	case <-p.done:
		return
	default:
		_ = p.command.Process.Kill()
		<-p.done
	}
}
func launch(t *testing.T, ctx context.Context, path string, env []string) *process {
	t.Helper()
	cmd := exec.CommandContext(ctx, path)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &process{command: cmd, done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(p.done) }()
	t.Cleanup(p.stop)
	return p
}
func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	return address
}
