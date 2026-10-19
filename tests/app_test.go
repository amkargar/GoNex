package tests

import (
	"context"
	"errors"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/examples/basic/modules/healthdemo"
	"github.com/amkargar/gonex/examples/basic/modules/users"
	"github.com/gin-gonic/gin"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type moduleFunc func(*app.Registrar) error

func (f moduleFunc) Register(r *app.Registrar) error { return f(r) }
func newApp(t *testing.T) *app.App {
	t.Helper()
	a, e := app.New(config.Config{Address: "127.0.0.1:0"})
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func TestModules(t *testing.T) {
	a := newApp(t)
	if e := a.Register(users.Module{}, healthdemo.Module{}); e != nil {
		t.Fatal(e)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/healthz", "/users", "/demo"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}
func TestRegistrationAtomicity(t *testing.T) {
	a := newApp(t)
	h := func(c *gin.Context) { c.Status(204) }
	err := a.Register(moduleFunc(func(r *app.Registrar) error {
		if e := r.Handle("GET", "/new", h); e != nil {
			return e
		}
		return r.Handle("GET", "/healthz", h)
	}))
	if err == nil {
		t.Fatal("duplicate accepted")
	}
	router, _ := a.Handler()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/new", nil))
	if w.Code != 404 {
		t.Fatal("partial registration")
	}
	if e := a.Register(moduleFunc(func(r *app.Registrar) error { return r.Handle("POST", "/healthz", h) })); e != nil {
		t.Fatal(e)
	}
	if e := a.Register(moduleFunc(func(r *app.Registrar) error { return r.Handle("GET", "bad", h) })); e == nil {
		t.Fatal("bad path accepted")
	}
}

type resource struct {
	id     string
	events *[]string
	fail   bool
}

func (r resource) Start(context.Context) error {
	*r.events = append(*r.events, "start:"+r.id)
	if r.fail {
		return errors.New("failed")
	}
	return nil
}
func (r resource) Close(context.Context) error {
	*r.events = append(*r.events, "close:"+r.id)
	return nil
}
func TestStartupRollback(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	a, _ := app.New(config.Config{Address: l.Addr().String()})
	events := []string{}
	if e = a.Register(moduleFunc(func(r *app.Registrar) error {
		if e := r.Manage(resource{"a", &events, false}); e != nil {
			return e
		}
		return r.Manage(resource{"b", &events, false})
	})); e != nil {
		t.Fatal(e)
	}
	if e = a.Start(context.Background()); e == nil {
		t.Fatal("occupied port accepted")
	}
	if !reflect.DeepEqual(events, []string{"start:a", "start:b", "close:b", "close:a"}) {
		t.Fatal(events)
	}
	if e = a.Shutdown(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestShutdownReleasesListener(t *testing.T) {
	a := newApp(t)
	if e := a.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	client := http.Client{Timeout: time.Second}
	res, e := client.Get("http://" + a.Address() + "/healthz")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(b) != "{\"status\":\"ok\"}" {
		t.Fatal(string(b))
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e = a.Shutdown(ctx); e != nil {
		t.Fatal(e)
	}
	if e = a.Shutdown(ctx); e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", a.Address())
	if e != nil {
		t.Fatal(e)
	}
	l.Close()
	if e = a.Start(ctx); e == nil {
		t.Fatal("restart accepted")
	}
	if e = a.Register(users.Module{}); e == nil {
		t.Fatal("late module accepted")
	}
}
func TestRunCancelled(t *testing.T) {
	a := newApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	deadline := time.After(2 * time.Second)
	for a.State() != app.Running {
		select {
		case <-deadline:
			t.Fatal("startup timed out")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-deadline:
		t.Fatal("shutdown hung")
	}
}
func TestConfigAndCancelledStartup(t *testing.T) {
	if _, e := app.New(config.Config{ReadTimeout: -1}); e == nil {
		t.Fatal("negative timeout accepted")
	}
	a := newApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := a.Start(ctx); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
