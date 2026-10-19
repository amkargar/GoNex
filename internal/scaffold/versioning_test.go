package scaffold

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGeneratedVersionMigration(t *testing.T) {
	root := project(t)
	p, e := CreateModule(root, "users", "http", false)
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	servicePath := filepath.Join(root, "modules/users/service.go")
	service, _ := os.ReadFile(servicePath)
	p, e = VersionModule(root, "users", "v1", "preserve", "")
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Apply(true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(root, "modules/users/http/v1")); !os.IsNotExist(e) {
		t.Fatal("version dry run wrote files")
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	v1Path := filepath.Join(root, "modules/users/http/v1/handlers.go")
	handwritten, _ := os.ReadFile(v1Path)
	handwritten = append(handwritten, []byte("// user customization\n")...)
	if e = os.WriteFile(v1Path, handwritten, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = VersionModule(root, "users", "v1", "preserve", ""); e == nil {
		t.Fatal("rerun did not fail")
	}
	p, e = VersionModule(root, "users", "v2", "preserve", "")
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	v2Path := filepath.Join(root, "modules/users/http/v2/schemas.go")
	v2, _ := os.ReadFile(v2Path)
	if e = os.WriteFile(v2Path, bytes.ReplaceAll(v2, []byte(`json:"message"`), []byte(`json:"message_v2"`)), 0644); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(servicePath)
	if !bytes.Equal(service, after) {
		t.Fatal("service changed")
	}
	after, _ = os.ReadFile(v1Path)
	if !bytes.Equal(handwritten, after) {
		t.Fatal("v1 customization changed")
	}
	smoke := `package main
import("testing";"net/http/httptest";"strings";"github.com/amkargar/gonex";"github.com/amkargar/gonex/config";"example.com/acme/shop/modules";"github.com/amkargar/gonex/openapi")
func TestVersions(t *testing.T){a,e:=gonex.New(config.Config{});if e!=nil{t.Fatal(e)};if e=a.Register(modules.All()...);e!=nil{t.Fatal(e)};h,e:=a.Handler();if e!=nil{t.Fatal(e)}
for _,path:=range []string{"/users","/api/v1/users","/api/v2/users"}{w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET",path,nil));if w.Code!=200{t.Fatal(path,w.Code)};if strings.Contains(path,"v2")!=strings.Contains(w.Body.String(),"message_v2"){t.Fatal(path,w.Body)}}
for _,v:=range []string{"v1","v2"}{w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/openapi/"+v+".json",nil));if w.Code!=200{t.Fatal(w.Code)};if e=openapi.Validate(w.Body.Bytes());e!=nil{t.Fatal(e)};if (v=="v2")!=strings.Contains(w.Body.String(),"message_v2"){t.Fatal(v)}}}
`
	if e = os.WriteFile(filepath.Join(root, "versions_test.go"), []byte(smoke), 0644); e != nil {
		t.Fatal(e)
	}
	framework, _ := filepath.Abs("../..")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for _, args := range [][]string{{"mod", "edit", "-replace=" + Framework + "=" + filepath.ToSlash(framework)}, {"mod", "tidy"}, {"test", "./..."}} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOWORK=off")
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("%v: %v\n%s", args, e, out)
		}
	}
}
func TestReplaceIsExplicit(t *testing.T) {
	root := project(t)
	p, _ := CreateModule(root, "users", "http", false)
	if e := p.Apply(false); e != nil {
		t.Fatal(e)
	}
	p, e := VersionModule(root, "users", "v1", "replace", "")
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Apply(false); e != nil {
		t.Fatal(e)
	}
	data, _ := os.ReadFile(filepath.Join(root, "modules/users/http_registry_gen.go"))
	if strings.Contains(string(data), "legacy") {
		t.Fatal("replace kept legacy registration")
	}
	if _, e = VersionModule(root, "users", "../v2", "preserve", ""); e == nil {
		t.Fatal("unsafe version accepted")
	}
}
