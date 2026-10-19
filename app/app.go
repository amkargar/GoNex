// Package app owns explicit registration and application lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/amkargar/gonex/config"
	transport "github.com/amkargar/gonex/http"
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
	err = a.registrar.Handle("GET", "/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	return a, err
}

func (a *App) State() State    { a.mu.Lock(); defer a.mu.Unlock(); return a.state }
func (a *App) Address() string { a.mu.Lock(); defer a.mu.Unlock(); return a.address }

func (a *App) Register(modules ...Module) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state != Created && a.state != Registered {
		return fmt.Errorf("cannot register in state %s", a.state)
	}
	r := &Registrar{routes: append([]transport.Route(nil), a.registrar.routes...), resources: append([]Resource(nil), a.registrar.resources...)}
	defer func() { r.sealed = true }()
	for _, m := range modules {
		if m == nil {
			return fmt.Errorf("nil module")
		}
		if err := m.Register(r); err != nil {
			return fmt.Errorf("register module: %w", err)
		}
	}
	a.registrar = Registrar{routes: r.routes, resources: r.resources}
	a.state = Registered
	return nil
}

// Handler returns a validated snapshot for embedding and transport tests.
func (a *App) Handler() (http.Handler, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return transport.Build(a.registrar.routes)
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
	h, err := transport.Build(a.registrar.routes)
	if err != nil {
		return fail(err)
	}
	for _, r := range a.registrar.resources {
		a.started = append(a.started, r)
		if err = r.Start(ctx); err != nil {
			return fail(fmt.Errorf("start resource: %w", err))
		}
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
