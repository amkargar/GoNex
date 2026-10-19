// Package app owns explicit registration and application lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"sync"

	"github.com/amkargar/gonex/config"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/openapi"
	"github.com/gin-gonic/gin"
)

type State string

const (
	Created    State = "created"
	Registered State = "registered"
	Starting   State = "starting"
	Running    State = "running"
	Stopping   State = "stopping"
	Stopped    State = "stopped"
)

type App struct {
	mu          sync.Mutex
	config      config.Config
	state       State
	registrar   Registrar
	server      *http.Server
	address     string
	started     []Resource
	serveErr    chan error
	done        chan struct{}
	shutdownErr error
}

func New(c config.Config) (*App, error) {
	c, err := c.Normalize()
	if err != nil {
		return nil, err
	}
	a := &App{config: c, state: Created, serveErr: make(chan error, 1), done: make(chan struct{})}
	a.registrar.reserved = map[string]bool{}
	if !c.DisableOpenAPI {
		a.registrar.reserved["GET /openapi.json"] = true
		if c.DocsEnabled {
			a.registrar.reserved["GET /docs"] = true
		}
	}
	err = transport.Register(&a.registrar, transport.Endpoint[transport.NoBody, HealthResponse]{Method: "GET", Path: "/healthz", OperationID: "health", Status: 200, Handler: func(context.Context, transport.NoBody) (HealthResponse, error) {
		return HealthResponse{Status: "ok"}, nil
	}})
	return a, err
}

type HealthResponse struct {
	Status string `json:"status"`
}

func (a *App) State() State    { a.mu.Lock(); defer a.mu.Unlock(); return a.state }
func (a *App) Address() string { a.mu.Lock(); defer a.mu.Unlock(); return a.address }

func (a *App) Register(modules ...Module) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state != Created && a.state != Registered {
		return fmt.Errorf("cannot register in state %s", a.state)
	}
	r := &Registrar{routes: append([]transport.Route(nil), a.registrar.routes...), resources: append([]Resource(nil), a.registrar.resources...), reserved: a.registrar.reserved}
	defer func() { r.sealed = true }()
	for _, m := range modules {
		if m == nil {
			return fmt.Errorf("nil module")
		}
		if err := m.Register(r); err != nil {
			return fmt.Errorf("register module: %w", err)
		}
	}
	a.registrar = Registrar{routes: r.routes, resources: r.resources, reserved: r.reserved}
	a.state = Registered
	return nil
}

// Handler returns a validated snapshot for embedding and transport tests.
func (a *App) Handler() (http.Handler, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.handler()
}

func (a *App) handler() (http.Handler, error) {
	routes := append([]transport.Route(nil), a.registrar.routes...)
	if !a.config.DisableOpenAPI {
		contracts := []openapi.Contract{}
		for _, r := range routes {
			if r.Contract != nil {
				contracts = append(contracts, *r.Contract)
			}
		}
		documents, err := openapi.Documents(contracts)
		if err != nil {
			return nil, err
		}
		versions := []string{}
		for version := range documents {
			versions = append(versions, version)
		}
		sort.Strings(versions)
		for _, version := range versions {
			path := "/openapi.json"
			if version != "" {
				path = "/openapi/" + version + ".json"
			}
			document := documents[version]
			routes = append(routes, transport.Route{Method: "GET", Path: path, Handlers: []gin.HandlerFunc{func(c *gin.Context) { c.Data(200, "application/json", document) }}})
		}
		if a.config.DocsEnabled {
			routes = append(routes, transport.Route{Method: "GET", Path: "/docs", Handlers: []gin.HandlerFunc{func(c *gin.Context) {
				c.Data(200, "text/html; charset=utf-8", []byte(`<!doctype html><html><title>Gonex API</title><h1>Gonex API</h1><p><a href="/openapi.json">OpenAPI 3.1 document</a></p></html>`))
			}}})
		}
	}
	return transport.Build(routes)
}

func (a *App) OpenAPI() ([]byte, error) {
	return a.OpenAPIVersion("")
}

func (a *App) OpenAPIVersion(version string) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	contracts := []openapi.Contract{}
	for _, r := range a.registrar.routes {
		if r.Contract != nil && r.Contract.Version == version {
			contracts = append(contracts, *r.Contract)
		}
	}
	return openapi.Generate(contracts)
}

func (a *App) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state != Created && a.state != Registered {
		return fmt.Errorf("cannot start in state %s", a.state)
	}
	a.state = Starting
	a.registrar.sealed = true
	fail := func(err error) error {
		cleanup, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
		defer cancel()
		err = errors.Join(err, a.closeResources(cleanup))
		a.state = Stopped
		close(a.done)
		return err
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	h, err := a.handler()
	if err != nil {
		return fail(err)
	}
	for _, r := range a.registrar.resources {
		a.started = append(a.started, r)
		if err = r.Start(ctx); err != nil {
			return fail(fmt.Errorf("start resource: %w", err))
		}
		if source, ok := r.(interface{ Errors() <-chan error }); ok {
			go func() {
				select {
				case err, open := <-source.Errors():
					if open && err != nil {
						select {
						case a.serveErr <- err:
						default:
						}
					}
				case <-a.done:
				}
			}()
		}
	}
	if a.config.DisableHTTP {
		a.state = Running
		return nil
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", a.config.Address)
	if err != nil {
		return fail(fmt.Errorf("listen HTTP: %w", err))
	}
	a.address = listener.Addr().String()
	a.server = &http.Server{Handler: h, ReadHeaderTimeout: a.config.ReadTimeout, ReadTimeout: a.config.ReadTimeout, WriteTimeout: a.config.WriteTimeout, IdleTimeout: a.config.IdleTimeout}
	a.state = Running
	go func() {
		err := a.server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		a.serveErr <- err
	}()
	return nil
}

func (a *App) Run(ctx context.Context) error {
	if err := a.Start(ctx); err != nil {
		return err
	}
	var err error
	select {
	case <-ctx.Done():
	case err = <-a.serveErr:
	case <-a.done:
	}
	stop, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
	defer cancel()
	return errors.Join(err, a.Shutdown(stop))
}

func (a *App) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	if a.state == Stopped || a.state == Stopping {
		done := a.done
		a.mu.Unlock()
		select {
		case <-done:
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.shutdownErr
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	a.state = Stopping
	server := a.server
	a.mu.Unlock()
	var err error
	if server != nil {
		err = server.Shutdown(ctx)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
	}
	err = errors.Join(err, a.closeResources(ctx))
	a.mu.Lock()
	a.shutdownErr = err
	a.state = Stopped
	close(a.done)
	a.mu.Unlock()
	return err
}

func (a *App) closeResources(ctx context.Context) error {
	var err error
	for i := len(a.started) - 1; i >= 0; i-- {
		err = errors.Join(err, a.started[i].Close(ctx))
	}
	a.started = nil
	return err
}
