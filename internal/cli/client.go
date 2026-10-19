package cli

import (
	"fmt"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/internal/contracts"
	"github.com/amkargar/gonex/internal/scaffold"
	"go/format"
	"path/filepath"
	"strings"
	"unicode"
)

func goName(s string) string {
	var b strings.Builder
	upper := true
	for _, r := range s {
		if r == '_' {
			upper = true
			continue
		}
		if upper {
			r = unicode.ToUpper(r)
			upper = false
		}
		b.WriteRune(r)
	}
	return b.String()
}
func (c invocation) clientPlan(root, provider, moduleName, version string) (*scaffold.Plan, error) {
	if err := scaffold.ValidName(provider); err != nil {
		return nil, err
	}
	if err := scaffold.ValidName(moduleName); err != nil {
		return nil, err
	}
	if !transport.ValidVersion(version) {
		return nil, fmt.Errorf("invalid contract version")
	}
	m, _, err := scaffold.Load(root)
	if err != nil {
		return nil, err
	}
	found := false
	for _, entry := range m.Modules {
		if entry.Name == moduleName {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("consumer module not found")
	}
	contractRoot, err := scaffold.ContractsPath(root)
	if err != nil {
		return nil, err
	}
	set, _, err := contracts.Compile(c.ctx, contractRoot)
	if err != nil {
		return nil, err
	}
	services := []string{}
	goPackage := ""
	for _, file := range set.File {
		if file.GetPackage() != provider+"."+version {
			continue
		}
		path := strings.Split(file.GetOptions().GetGoPackage(), ";")[0]
		if goPackage != "" && path != goPackage {
			return nil, fmt.Errorf("selected provider contract uses multiple Go packages; select a single API")
		}
		goPackage = path
		for _, s := range file.Service {
			services = append(services, goName(s.GetName()))
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no service contracts found for %s.%s", provider, version)
	}
	title := goName(provider)
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\nimport (rpc %q;pb %q)\n// %sClient exposes generated clients. TODO: implement your local business interface\n// by mapping plain Go inputs/results to these exact contract messages.\ntype %sClient struct{\n", moduleName, scaffold.Framework+"/grpc", goPackage, title, title)
	for _, s := range services {
		fmt.Fprintf(&b, "%s pb.%sClient\n", s, s)
	}
	fmt.Fprintf(&b, "}\nfunc New%sClient(pool *rpc.Clients,options rpc.ClientOptions)(*%sClient,error){config,err:=rpc.NewClientConfig(options);if err!=nil{return nil,err};result:= &%sClient{}\n", title, title, title)
	for _, s := range services {
		fmt.Fprintf(&b, "result.%s,err=rpc.Client(pool,config,pb.New%sClient);if err!=nil{return nil,err}\n", s, s)
	}
	b.WriteString("return result,nil\n}\n")
	adapter, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, err
	}
	defaultAddress := ""
	owner, _, e := scaffold.Load(filepath.Dir(contractRoot))
	if e == nil {
		for _, s := range owner.Services {
			if s.Name == provider {
				defaultAddress = s.GRPCAddress
			}
		}
	}
	prefix := "CLIENTS_" + strings.ToUpper(provider)
	settings := fmt.Sprintf(`package config
import("crypto/tls";"os";"time";rpc %q)
type %sClientSettings struct{Address string;TLS *tls.Config;DevelopmentInsecure bool;Timeout time.Duration}
func(s %sClientSettings)Options()rpc.ClientOptions{return rpc.ClientOptions{Target:s.Address,TLS:s.TLS,DevelopmentInsecure:s.DevelopmentInsecure,Timeout:s.Timeout}}
func Load%sClient()(%sClientSettings,error){tlsConfig,err:=rpc.ClientTLS(os.Getenv(%q),os.Getenv(%q),os.Getenv(%q),os.Getenv(%q));if err!=nil{return %sClientSettings{},err};address:=os.Getenv(%q);if address==""{address=%q};return %sClientSettings{Address:address,TLS:tlsConfig,DevelopmentInsecure:os.Getenv(%q)=="1",Timeout:5*time.Second},nil}
`, scaffold.Framework+"/grpc", title, title, title, title, prefix+"_CA", prefix+"_SERVER_NAME", prefix+"_CERT", prefix+"_KEY", title, prefix+"_ADDRESS", defaultAddress, title, prefix+"_INSECURE")
	config, err := format.Source([]byte(settings))
	if err != nil {
		return nil, err
	}
	p := &scaffold.Plan{Root: root, Changes: []scaffold.Change{{Path: filepath.Join("modules", moduleName, provider+"_client.go"), Data: adapter}, {Path: filepath.Join("config", "clients_"+provider+".go"), Data: config}}}
	if err = scaffold.AddDependency(p, m.ContractsModule); err != nil {
		return nil, err
	}
	return p, nil
}
