// Package app owns explicit registration and application lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/amkargar/gonex/internal/buildinfo"
	"github.com/amkargar/gonex/observability"
	"log/slog"
	"net"
	"net/http"
	"net/url"
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
	listener    net.Listener
	address     string
	started     []Resource
	serveErr    chan error
	done        chan struct{}
	shutdownErr error
	startDone   chan struct{}
	startCancel context.CancelFunc
}

func New(c config.Config) (*App, error) {
	c, err := c.Normalize()
	if err != nil {
		return nil, err
	}
	a := &App{config: c, state: Created, serveErr: make(chan error, 1), done: make(chan struct{}), startDone: make(chan struct{})}
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
	if err == nil {
		err = transport.Register(&a.registrar, transport.Endpoint[transport.NoBody, HealthResponse]{Method: "GET", Path: "/readyz", OperationID: "readiness", Status: 200, Errors: []int{503}, Handler: func(ctx context.Context, _ transport.NoBody) (HealthResponse, error) {
			if err := a.Ready(ctx); err != nil {
				return HealthResponse{}, err
			}
			return HealthResponse{Status: "ready"}, nil
		}})
	}
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
	r := &Registrar{routes: append([]transport.Route(nil), a.registrar.routes...), resources: append([]Resource(nil), a.registrar.resources...), reserved: a.registrar.reserved, checks: append([]readinessCheck(nil), a.registrar.checks...)}
	defer func() { r.sealed = true }()
	for _, m := range modules {
		if m == nil {
			return fmt.Errorf("nil module")
		}
		if err := m.Register(r); err != nil {
			return fmt.Errorf("register module: %w", err)
		}
	}
	a.registrar = Registrar{routes: r.routes, resources: r.resources, reserved: r.reserved, checks: r.checks}
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
	middleware := append([]gin.HandlerFunc(nil), a.config.Middleware...)
	if !a.config.DisableRequestLogging {
		middleware = append([]gin.HandlerFunc{observability.RequestLog(a.config.Logger)}, middleware...)
	}
	return transport.Build(routes, middleware...)
}

func (a *App) OpenAPI() ([]byte, error) {
	return a.OpenAPIVersion("")
}

func (a *App) OpenAPIVersion(version string) ([]byte, error) {
	if version != "" && !transport.ValidVersion(version) {
		return nil, fmt.Errorf("invalid HTTP version")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	contracts := []openapi.Contract{}
	for _, r := range a.registrar.routes {
		if r.Contract != nil && r.Contract.Version == version {
			contracts = append(contracts, *r.Contract)
		}
	}
	if version != "" && len(contracts) == 0 {
		return nil, fmt.Errorf("HTTP version %s is not registered", version)
	}
	return openapi.Generate(contracts)
}

func (a *App) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.state != Created && a.state != Registered {
		state := a.state
		a.mu.Unlock()
		return fmt.Errorf("cannot start in state %s", state)
	}
	startCtx, cancel := context.WithCancel(ctx)
	a.startCancel = cancel
	a.state = Starting
	a.registrar.sealed = true
	resources := append([]Resource(nil), a.registrar.resources...)
	a.mu.Unlock()
	defer close(a.startDone)
	fail := func(err error) error {
		cancel()
		cleanup, stop := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
		defer stop()
		cleanupErr := a.closeResources(cleanup)
		a.mu.Lock()
		a.shutdownErr = cleanupErr
		a.state = Stopped
		close(a.done)
		a.mu.Unlock()
		a.config.Logger.ErrorContext(ctx, "server startup failed", "error", "startup_failed")
		return errors.Join(err, cleanupErr)
	}
	if err := startCtx.Err(); err != nil {
		return fail(err)
	}
	// Registration is immutable in Starting; callbacks run without the state mutex.
	h, err := a.handler()
	if err != nil {
		return fail(err)
	}
	for _, r := range resources {
		a.started = append(a.started, r)
		if err = r.Start(startCtx); err != nil {
			return fail(fmt.Errorf("start resource: %w", err))
		}
		if err = startCtx.Err(); err != nil {
			return fail(err)
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
		a.mu.Lock()
		a.state = Running
		a.mu.Unlock()
		a.logStarted(ctx)
		return nil
	}
	listener, err := (&net.ListenConfig{}).Listen(startCtx, "tcp", a.config.Address)
	if err != nil {
		return fail(fmt.Errorf("listen HTTP: %w", err))
	}
	server := &http.Server{Handler: h, ReadHeaderTimeout: a.config.ReadTimeout, ReadTimeout: a.config.ReadTimeout, WriteTimeout: a.config.WriteTimeout, IdleTimeout: a.config.IdleTimeout}
	a.mu.Lock()
	a.address = listener.Addr().String()
	a.server = server
	a.listener = listener
	a.state = Running
	a.mu.Unlock()
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		select {
		case a.serveErr <- err:
		default:
		}
	}()
	a.logStarted(ctx)
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
	if err != nil {
		a.config.Logger.ErrorContext(ctx, "server failed", "error", "resource_failed")
	}
	return errors.Join(err, a.Shutdown(stop))
}

func (a *App) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	if a.state == Starting {
		cancel, done := a.startCancel, a.startDone
		a.mu.Unlock()
		cancel()
		select {
		case <-done:
			return a.Shutdown(ctx)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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
	listener := a.listener
	a.mu.Unlock()
	a.config.Logger.InfoContext(ctx, "server stopping")
	var err error
	if server != nil {
		err = server.Shutdown(ctx)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
	}
	if listener != nil {
		_ = listener.Close()
	}
	if a.startCancel != nil {
		a.startCancel()
	}
	err = errors.Join(err, a.closeResources(ctx))
	a.mu.Lock()
	a.shutdownErr = err
	a.state = Stopped
	close(a.done)
	a.mu.Unlock()
	if err != nil {
		a.config.Logger.ErrorContext(ctx, "server shutdown failed", "error", "shutdown_failed")
	} else {
		a.config.Logger.InfoContext(ctx, "server stopped")
	}
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

// Logger is shared by lifecycle events and application code. It is never global.
func (a *App) Logger() *slog.Logger { return a.config.Logger }

func (a *App) logStarted(ctx context.Context) {
	if a.config.DisableStartupBanner {
		return
	}
	attrs := []any{"name", a.config.Name, "version", buildinfo.Version()}
	if !a.config.DisableHTTP {
		host, port, _ := net.SplitHostPort(a.address)
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "localhost"
		}
		local := (&url.URL{Scheme: "http", Host: net.JoinHostPort(host, port)}).String()
		attrs = append(attrs, "url", local, "listen", a.address)
		if !a.config.DisableOpenAPI {
			attrs = append(attrs, "openapi", local+"/openapi.json")
			seen := map[string]bool{}
			for _, r := range a.registrar.routes {
				if r.Contract != nil && r.Contract.Version != "" && !seen[r.Contract.Version] {
					v := r.Contract.Version
					seen[v] = true
					attrs = append(attrs, "openapi_"+v, local+"/openapi/"+v+".json")
				}
			}
			if a.config.DocsEnabled {
				attrs = append(attrs, "docs", local+"/docs")
			}
		}
	}
	attrs = append(attrs, "stop", "Ctrl+C")
	a.config.Logger.InfoContext(ctx, "Gonex API server ready", attrs...)
}
