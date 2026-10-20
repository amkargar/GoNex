package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amkargar/gonex/internal/scaffold"
	"github.com/amkargar/gonex/internal/workspace"
)

func dxWorkspace(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "commerce")
	p, e := scaffold.NewMicroservices(root, "example.com/acme/commerce", "v0.0.0")
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	for _, n := range []string{"identity", "orders"} {
		p, e = scaffold.CreateServiceWithTransports(root, n, n == "orders", n == "identity")
		if e != nil {
			t.Fatal(e)
		}
		if e = p.Apply(false); e != nil {
			t.Fatal(e)
		}
	}
	p, e = scaffold.CreateModule(filepath.Join(root, "services/orders"), "orders", "http", false)
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{"v1", "v2"} {
		d := filepath.Join(root, "contracts/proto/identity", v)
		os.MkdirAll(d, 0755)
		s := `syntax="proto3"; package identity.` + v + `;option go_package="example.com/acme/commerce/contracts/gen/go/identity/` + v + `;identity` + v + `";service UsersService{rpc GetUsersMessage(Request)returns(Response);}message Request{}message Response{string message=1;}`
		os.WriteFile(filepath.Join(d, "users.proto"), []byte(s), 0644)
	}
	return root
}
func TestClientProfilesAndTwoVersions(t *testing.T) {
	for _, profile := range []string{"skeleton", "example", "raw"} {
		t.Run(profile, func(t *testing.T) {
			root := dxWorkspace(t)
			var output bytes.Buffer
			args := []string{"create", "client", "identity", "--service", "orders", "--module", "orders", "--profile", profile}
			if code := Execute(context.Background(), args, root, &output, &output); code != 0 {
				t.Fatal(output.String())
			}
			path := filepath.Join(root, "services/orders/modules/orders/identity_client.go")
			b, _ := os.ReadFile(path)
			if strings.Contains(string(b), "GONEX_TODO_") != (profile == "skeleton") {
				t.Fatal("truthful mapping marker missing")
			}
			if profile == "example" && !strings.Contains(string(b), "GetUsersMessage(ctx, &pb.Request{})") {
				t.Fatal("descriptor mapping incorrect", string(b))
			}
			if code := Execute(context.Background(), append(args, "--version", "v2"), root, &output, &output); code != 0 {
				t.Fatal(output.String())
			}
			if _, e := os.Stat(filepath.Join(root, "services/orders/modules/orders/identity_v2_client.go")); e != nil {
				t.Fatal(e)
			}
			if code := Execute(context.Background(), args, root, &output, &output); code == 0 {
				t.Fatal("regeneration overwrote caller-owned adapter")
			}
		})
	}
}
func TestClientExampleRejectsInventedBusinessMapping(t *testing.T) {
	root := dxWorkspace(t)
	path := filepath.Join(root, "contracts/proto/identity/v1/users.proto")
	b, _ := os.ReadFile(path)
	os.WriteFile(path, bytes.Replace(b, []byte("message Request{}"), []byte("message Request{string id=1;}"), 1), 0644)
	var out bytes.Buffer
	code := Execute(context.Background(), []string{"create", "client", "identity", "--service", "orders", "--module", "orders", "--profile", "example"}, root, &out, &out)
	if code == 0 || !strings.Contains(out.String(), "skeleton") {
		t.Fatal("invented semantic mapping", out.String())
	}
	if _, e := os.Stat(filepath.Join(root, "services/orders/modules/orders/identity_client.go")); !os.IsNotExist(e) {
		t.Fatal("failed generation wrote files")
	}
}
func TestServiceFlagsAndDryRun(t *testing.T) {
	root := dxWorkspace(t)
	before, _ := os.ReadFile(filepath.Join(root, "gonex.yaml"))
	var out bytes.Buffer
	for _, args := range [][]string{{"create", "service", "mixed", "--http", "--grpc", "--dry-run"}, {"create", "service", "worker", "--transport", "none", "--dry-run"}} {
		if Execute(context.Background(), args, root, &out, &out) != 0 {
			t.Fatal(out.String())
		}
	}
	after, _ := os.ReadFile(filepath.Join(root, "gonex.yaml"))
	if !bytes.Equal(before, after) {
		t.Fatal("dry-run wrote catalog")
	}
	if _, e := os.Stat(filepath.Join(root, "services/mixed")); !os.IsNotExist(e) {
		t.Fatal("dry-run wrote service")
	}
	for _, args := range [][]string{{"create", "service", "bad", "--http", "--transport", "grpc"}, {"create", "service", "../escape"}, {"create", "module", "bad", "--profile", "example"}, {"create", "client", "identity", "--service", "orders", "--module", "orders", "--db"}} {
		if Execute(context.Background(), args, root, &out, &out) == 0 {
			t.Fatal("invalid flags accepted", args)
		}
	}
}
func TestDoctorJSONDoesNotLeakSecrets(t *testing.T) {
	root := dxWorkspace(t)
	os.RemoveAll(filepath.Join(root, "contracts/proto"))
	t.Setenv("GRPC_TLS_KEY", "PRIVATE_SECRET_KEY_PATH")
	t.Setenv("GRPC_TLS_CERT", "PRIVATE_SECRET_CERT_PATH")
	var out, errout bytes.Buffer
	code := Execute(context.Background(), []string{"doctor", "--json"}, root, &out, &errout)
	if code == 0 {
		t.Fatal("invalid TLS accepted")
	}
	var report any
	if json.Unmarshal(out.Bytes(), &report) != nil {
		t.Fatal("doctor did not produce JSON", out.String())
	}
	if strings.Contains(out.String()+errout.String(), "PRIVATE_SECRET") {
		t.Fatal("secret leaked")
	}
}
func TestCatalogProviderDependency(t *testing.T) {
	root := dxWorkspace(t)
	var out bytes.Buffer
	if Execute(context.Background(), []string{"create", "client", "identity", "--service", "orders", "--module", "orders", "--profile", "example"}, root, &out, &out) != 0 {
		t.Fatal(out.String())
	}
	cat, _, e := workspace.Load(root)
	if e != nil || cat.Services["orders"].Clients["identity"].Address == "" {
		t.Fatal("client edge missing", e)
	}
	p, e := workspace.Resolve(cat, root, "orders", true, func(string) string { return "" })
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, i := range p.Issues {
		if i.Code == "wiring_missing" {
			found = true
		}
	}
	if !found {
		t.Fatal("unwired adapter not diagnosed")
	}
}
