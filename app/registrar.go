package app

import (
	"fmt"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/openapi"
	"github.com/gin-gonic/gin"
	"strings"
)

type Registrar struct {
	checks    []readinessCheck
	routes    []transport.Route
	resources []Resource
	sealed    bool
	reserved  map[string]bool
}

func (r *Registrar) Handle(method, path string, handlers ...gin.HandlerFunc) error {
	return r.Endpoint(transport.Route{Method: strings.ToUpper(method), Path: path, Handlers: append([]gin.HandlerFunc(nil), handlers...)})
}

func (r *Registrar) Endpoint(candidate transport.Route) error {
	if candidate.Contract != nil {
		contract, err := candidate.Contract.Freeze()
		if err != nil {
			return err
		}
		if candidate.Method != contract.Method || candidate.Path != contract.Path {
			return fmt.Errorf("HTTP route does not match its typed contract")
		}
		candidate.Contract = &contract
	}
	candidate.Handlers = append([]gin.HandlerFunc(nil), candidate.Handlers...)
	if r.reserved["GET /openapi.json"] && candidate.Method == "GET" && strings.HasPrefix(candidate.Path, "/openapi/") {
		return fmt.Errorf("OpenAPI document paths are owned by runtime")
	}
	if r.reserved[candidate.Method+" "+candidate.Path] {
		return fmt.Errorf("route is owned by runtime: %s %s", candidate.Method, candidate.Path)
	}
	if r.sealed {
		return fmt.Errorf("registration is closed")
	}
	routes := append(append([]transport.Route(nil), r.routes...), candidate)
	if _, err := transport.Build(routes); err != nil {
		return err
	}
	contracts := []openapi.Contract{}
	for _, route := range routes {
		if route.Contract != nil {
			contracts = append(contracts, *route.Contract)
		}
	}
	if _, err := openapi.Generate(contracts); err != nil {
		return err
	}
	r.routes = routes
	return nil
}

func (r *Registrar) Manage(resource Resource) error {
	if r.sealed {
		return fmt.Errorf("registration is closed")
	}
	if resource == nil {
		return fmt.Errorf("nil resource")
	}
	r.resources = append(r.resources, resource)
	return nil
}
