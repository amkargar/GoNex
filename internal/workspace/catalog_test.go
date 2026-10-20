package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogStrictLocalAndDeterministic(t *testing.T) {
	good := "schema: 1\nservices:\n  identity:\n    path: services/identity\n    grpc: 127.0.0.1:9091\n  orders:\n    path: services/orders\n    http: 127.0.0.1:8082\n    clients:\n      identity:\n        address: 127.0.0.1:9091\n        transport: grpc\n"
	c, e := Parse([]byte(good))
	if e != nil {
		t.Fatal(e)
	}
	one, _ := c.Marshal()
	d, e := Parse(one)
	if e != nil {
		t.Fatal(e)
	}
	two, _ := d.Marshal()
	if string(one) != string(two) {
		t.Fatal("nondeterministic catalog")
	}
	for _, bad := range []string{strings.Replace(good, "schema: 1", "schema: 2", 1), good + "password: SECRET\n", good + "---\nschema: 1\n", strings.Replace(good, "127.0.0.1:8082", "localhost:9091", 1), strings.Replace(good, "services/orders", "../escape", 1), strings.Replace(good, "127.0.0.1:9091", "0.0.0.0:9091", 1), strings.Replace(good, "path: services/orders", "path: services/orders\n    path: services/other", 1), strings.Replace(good, "services/orders", "services\\orders", 1)} {
		if _, e = Parse([]byte(bad)); e == nil {
			t.Fatal("accepted invalid catalog")
		}
		if e != nil && strings.Contains(e.Error(), "SECRET") {
			t.Fatal("diagnostic echoed secret")
		}
	}
}
func TestResolveOverridesAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"identity", "orders"} {
		d := filepath.Join(root, "services", n)
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, "go.mod"), []byte("module example.com/"+n+"\n"), 0644)
	}
	c := Catalog{Schema: 1, Services: map[string]Service{"identity": {Path: "services/identity", GRPC: "127.0.0.1:49091"}, "orders": {Path: "services/orders", HTTP: "127.0.0.1:48082", Clients: map[string]Client{"identity": {Address: "127.0.0.1:49091", Transport: "grpc"}}}}}
	lookup := func(k string) string {
		if k == "GONEX_SERVICES_IDENTITY_GRPC_ADDRESS" {
			return "127.0.0.1:49092"
		}
		return ""
	}
	p, e := Resolve(c, root, "", true, lookup)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(p.Environment["orders"], "\n"), "CLIENTS_IDENTITY_ADDRESS=127.0.0.1:49092") {
		t.Fatal("provider override did not propagate")
	}
	if c.Services["identity"].GRPC != "127.0.0.1:49091" {
		t.Fatal("mutated input")
	}
	if _, e = Resolve(c, root, "", true, func(k string) string {
		if k == "GONEX_ENV" {
			return "production"
		}
		return ""
	}); e == nil {
		t.Fatal("production plaintext accepted")
	}
	os.WriteFile(filepath.Join(root, "services/orders/todo.go"), []byte("package orders\nvar _ = GONEX_TODO_MAPPING"), 0644)
	p, e = Resolve(c, root, "orders", true, func(string) string { return "" })
	if e != nil || !p.HasErrors() {
		t.Fatal("mapping diagnostics missing", e)
	}
}
func TestCyclesAreWarnings(t *testing.T) {
	c := Catalog{Services: map[string]Service{"a": {Clients: map[string]Client{"b": {}}}, "b": {Clients: map[string]Client{"a": {}}}}}
	if len(c.Cycles()) != 1 {
		t.Fatal("cycle not detected")
	}
}

func TestSingleServiceGlobalOverrideDoesNotInventListener(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "services/orders")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/orders\n"), 0644)
	c := Catalog{Schema: 1, Services: map[string]Service{"orders": {Path: "services/orders", HTTP: "127.0.0.1:48081"}}}
	p, e := Resolve(c, root, "orders", false, func(k string) string {
		if k == "GRPC_ADDRESS" {
			return "127.0.0.1:49091"
		}
		return ""
	})
	if e != nil {
		t.Fatal(e)
	}
	if p.Catalog.Services["orders"].GRPC != "" {
		t.Fatal("global override invented a disabled listener")
	}
}
