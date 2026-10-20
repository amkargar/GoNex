package http

import (
	"context"
	"fmt"

	"github.com/amkargar/gonex/openapi"
	"github.com/gin-gonic/gin"
)

// RouteDefinition is a typed route declaration accepted by Routes.
type RouteDefinition interface {
	register(EndpointRegistrar) error
}

// TypedRoute keeps request/response inference while describing an endpoint.
type TypedRoute[I, O any] struct {
	endpoint Endpoint[I, O]
}

func route[I, O any](method, path string, handler func(context.Context, I) (O, error)) *TypedRoute[I, O] {
	return &TypedRoute[I, O]{endpoint: Endpoint[I, O]{Method: method, Path: path, Status: 200, Handler: handler}}
}

func Get[I, O any](path string, handler func(context.Context, I) (O, error)) *TypedRoute[I, O] {
	return route("GET", path, handler)
}

func Post[I, O any](path string, handler func(context.Context, I) (O, error)) *TypedRoute[I, O] {
	return route("POST", path, handler).Status(201)
}

func Put[I, O any](path string, handler func(context.Context, I) (O, error)) *TypedRoute[I, O] {
	return route("PUT", path, handler)
}

func Patch[I, O any](path string, handler func(context.Context, I) (O, error)) *TypedRoute[I, O] {
	return route("PATCH", path, handler)
}

func Delete[I, O any](path string, handler func(context.Context, I) (O, error)) *TypedRoute[I, O] {
	return route("DELETE", path, handler).Status(204)
}

func Head[I, O any](path string, handler func(context.Context, I) (O, error)) *TypedRoute[I, O] {
	return route("HEAD", path, handler)
}

func Options[I, O any](path string, handler func(context.Context, I) (O, error)) *TypedRoute[I, O] {
	return route("OPTIONS", path, handler)
}

func (r *TypedRoute[I, O]) Name(name string) *TypedRoute[I, O] {
	r.endpoint.OperationID = name
	return r
}

func (r *TypedRoute[I, O]) Summary(summary string) *TypedRoute[I, O] {
	r.endpoint.Summary = summary
	return r
}

func (r *TypedRoute[I, O]) Description(description string) *TypedRoute[I, O] {
	r.endpoint.Description = description
	return r
}

func (r *TypedRoute[I, O]) Tags(tags ...string) *TypedRoute[I, O] {
	r.endpoint.Tags = append([]string(nil), tags...)
	return r
}

func (r *TypedRoute[I, O]) Status(status int) *TypedRoute[I, O] {
	r.endpoint.Status = status
	return r
}

func (r *TypedRoute[I, O]) Errors(statuses ...int) *TypedRoute[I, O] {
	r.endpoint.Errors = append([]int(nil), statuses...)
	return r
}

func (r *TypedRoute[I, O]) Use(middleware ...gin.HandlerFunc) *TypedRoute[I, O] {
	r.endpoint.Middleware = append(r.endpoint.Middleware, middleware...)
	return r
}

func (r *TypedRoute[I, O]) Security(requirements []map[string][]string, schemes openapi.Object) *TypedRoute[I, O] {
	r.endpoint.Security = requirements
	r.endpoint.SecuritySchemes = schemes
	return r
}

func (r *TypedRoute[I, O]) MaxBodyBytes(limit int64) *TypedRoute[I, O] {
	r.endpoint.MaxBodyBytes = limit
	return r
}

func (r *TypedRoute[I, O]) Examples(request, response any) *TypedRoute[I, O] {
	r.endpoint.RequestExample = request
	r.endpoint.ResponseExample = response
	return r
}

func (r *TypedRoute[I, O]) register(registrar EndpointRegistrar) error {
	if r == nil {
		return fmt.Errorf("nil route definition")
	}
	return Register(registrar, r.endpoint)
}

// Routes registers declarations in order through the existing typed pipeline.
// Return its error from Module.Register so application registration stays atomic.
func Routes(registrar EndpointRegistrar, definitions ...RouteDefinition) error {
	if registrar == nil {
		return fmt.Errorf("nil route registrar")
	}
	for _, definition := range definitions {
		if definition == nil {
			return fmt.Errorf("nil route definition")
		}
		if err := definition.register(registrar); err != nil {
			return err
		}
	}
	return nil
}
