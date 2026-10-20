package cli

import (
	"fmt"

	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/internal/contracts"
	"github.com/amkargar/gonex/internal/scaffold"
	"github.com/amkargar/gonex/internal/workspace"
	"go/format"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
	"os"
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
func (c invocation) clientProfilePlan(root, provider, moduleName, version, profile, selectedRPC string) (*scaffold.Plan, error) {
	if profile != "skeleton" && profile != "example" && profile != "raw" {
		return nil, fmt.Errorf("client --profile must be skeleton, example or raw")
	}
	if selectedRPC != "" && profile != "example" {
		return nil, fmt.Errorf("--rpc is only used with --profile example")
	}
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
	generated, err := (protogen.Options{}).New(&pluginpb.CodeGeneratorRequest{ProtoFile: set.File})
	if err != nil {
		return nil, fmt.Errorf("invalid generated Go contract packages")
	}
	goServices := map[string]*protogen.Service{}
	goMethods := map[string]*protogen.Method{}
	for _, f := range generated.Files {
		for _, s := range f.Services {
			goServices[string(s.Desc.FullName())] = s
			for _, m := range s.Methods {
				goMethods[string(m.Desc.FullName())] = m
			}
		}
	}
	services := []string{}
	type method struct {
		service string
		rpc     *descriptorpb.MethodDescriptorProto
	}
	var methods []method
	messages := map[string]*descriptorpb.DescriptorProto{}
	for _, file := range set.File {
		for _, msg := range file.MessageType {
			messages["."+file.GetPackage()+"."+msg.GetName()] = msg
		}
	}
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
			services = append(services, goServices[file.GetPackage()+"."+s.GetName()].GoName)
			for _, rpc := range s.Method {
				methods = append(methods, method{s.GetName(), rpc})
			}
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no service contracts found for %s.%s", provider, version)
	}
	title := goName(provider)
	suffix := ""
	if version != "v1" {
		suffix = "_" + version
		title += goName(version)
	}
	var chosen method
	if profile == "example" {
		var matches []method
		for _, candidate := range methods {
			if selectedRPC == "" || selectedRPC == candidate.service+"."+candidate.rpc.GetName() {
				matches = append(matches, candidate)
			}
		}
		if len(matches) != 1 {
			return nil, fmt.Errorf("example requires one unambiguous RPC; select --rpc Service.Method")
		}
		chosen = matches[0]
		req, res := messages[chosen.rpc.GetInputType()], messages[chosen.rpc.GetOutputType()]
		if chosen.rpc.GetClientStreaming() || chosen.rpc.GetServerStreaming() || req == nil || res == nil || len(req.Field) != 0 || len(res.Field) != 1 || res.Field[0].GetName() != "message" || res.Field[0].GetNumber() != 1 || res.Field[0].GetType() != descriptorpb.FieldDescriptorProto_TYPE_STRING || res.Field[0].GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			return nil, fmt.Errorf("example only supports an empty request and exactly string message = 1; use skeleton for business contracts")
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "package %s\nimport (\"context\";rpc %q;pb %q)\n// %sClient exposes generated clients. TODO: implement your local business interface\n// by mapping plain Go inputs/results to these exact contract messages.\ntype %sClient struct{pool *rpc.Clients;config *rpc.ClientConfig\n", moduleName, scaffold.Framework+"/grpc", goPackage, title, title)
	for _, s := range services {
		fmt.Fprintf(&b, "%s pb.%sClient\n", s, s)
	}
	fmt.Fprintf(&b, "}\nfunc New%sClient(pool *rpc.Clients,options rpc.ClientOptions)(*%sClient,error){config,err:=rpc.NewClientConfig(options);if err!=nil{return nil,err};return New%sClientWithConfig(pool,config)}\nfunc New%sClientWithConfig(pool *rpc.Clients,config *rpc.ClientConfig)(*%sClient,error){var err error;result:= &%sClient{pool:pool,config:config}\n", title, title, title, title, title, title)
	for _, s := range services {
		fmt.Fprintf(&b, "result.%s,err=rpc.Client(pool,config,pb.New%sClient);if err!=nil{return nil,err}\n", s, s)
	}
	b.WriteString("return result,nil\n}\n")
	fmt.Fprintf(&b, "func(c *%sClient)Ready(ctx context.Context)error{return c.pool.Ready(ctx,c.config)}\n", title)
	if profile == "example" {
		method := goMethods[provider+"."+version+"."+chosen.service+"."+chosen.rpc.GetName()]
		in := method.Input.GoIdent.GoName
		fmt.Fprintf(&b, "// Message implements only the explicitly selected demonstration contract.\nfunc(c *%sClient)Message(ctx context.Context)(string,error){res,err:=c.%s.%s(ctx,&pb.%s{});if err!=nil{return \"\",rpc.FromStatus(err)};return res.GetMessage(),nil}\n", title, goServices[provider+"."+version+"."+chosen.service].GoName, method.GoName, in)
	}
	if profile == "skeleton" {
		fmt.Fprintf(&b, "// GONEX_TODO: implement your local interface and explicit field/error mapping here.\n// Remove the following compile-time marker only after mapping and wiring the adapter.\nvar _ = GONEX_TODO_%s_%s_MAPPING_AND_WIRING\n", provider, version)
	}

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
	p := &scaffold.Plan{Root: root, Changes: []scaffold.Change{{Path: filepath.Join("modules", moduleName, provider+suffix+"_client.go"), Data: adapter}, {Path: filepath.Join("config", "clients_"+provider+suffix+".go"), Data: config}}}
	// Dependency publication/version selection is explicit; local contracts
	// resolve through go.work without a fake release requirement.
	bootstrap := fmt.Sprintf(`package %s
import("context";"time";"%s/app";rpc "%s/grpc";settings "%s/config")
// New%sForModule is called once from the caller-owned NewModuleService factory.
// GONEX_WIRING: assign the returned adapter to your plain-Go business interface.
func New%sForModule(r *app.Registrar)(result *%sClient,err error){options,err:=settings.Load%sClient();if err!=nil{return nil,err};pool:=rpc.NewClients();defer func(){if err!=nil{ctx,cancel:=context.WithTimeout(context.Background(),time.Second);defer cancel();_ = pool.Close(ctx)}}();client,err:=New%sClient(pool,options.Options());if err!=nil{return nil,err};if err=r.Manage(pool);err!=nil{return nil,err};if err=r.Check(%q,func(ctx context.Context)error{return client.Ready(ctx)});err!=nil{return nil,err};return client,nil}
`, moduleName, scaffold.Framework, scaffold.Framework, m.Module, title, title, title, title, title, provider+"_"+version)
	if profile != "raw" {
		bootstrap = strings.Replace(bootstrap, "GONEX_WIRING:", "GONEX_WIRING_REQUIRED:", 1)
	}
	boot, err := format.Source([]byte(bootstrap))
	if err != nil {
		return nil, err
	}
	p.Changes = append(p.Changes, scaffold.Change{Path: filepath.Join("modules", moduleName, provider+suffix+"_bootstrap.go"), Data: boot})
	if err = rebaseWorkspace(p); err != nil {
		return nil, err
	}
	if owner, _, e := scaffold.Load(p.Root); e == nil && owner.Architecture == "microservices" {
		cat, before, e := workspace.Load(p.Root)
		if e == nil {
			entry := cat.Services[m.Service]
			if entry.Clients == nil {
				entry.Clients = map[string]workspace.Client{}
			}
			target, ok := cat.Services[provider]
			if !ok || target.GRPC == "" {
				return nil, fmt.Errorf("provider needs a declared gRPC listener in gonex.yaml")
			}
			entry.Clients[provider] = workspace.Client{Address: target.GRPC, Transport: "grpc"}
			cat.Services[m.Service] = entry
			data, e := cat.Marshal()
			if e != nil {
				return nil, e
			}
			p.Changes = append(p.Changes, scaffold.Change{Path: workspace.Filename, Data: data, Before: before, Update: true})
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}

	return p, nil
}
