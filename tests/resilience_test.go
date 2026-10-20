package tests

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
)

type startingResource struct {
	entered chan struct{}
	closed  chan struct{}
}

func (r startingResource) Start(ctx context.Context) error {
	close(r.entered)
	<-ctx.Done()
	return ctx.Err()
}
func (r startingResource) Close(context.Context) error { close(r.closed); return nil }
func TestShutdownWhileStartingDoesNotDeadlock(t *testing.T) {
	r := startingResource{make(chan struct{}), make(chan struct{})}
	a := newApp(t)
	if err := a.Register(moduleFunc(func(reg *app.Registrar) error { return reg.Manage(r) })); err != nil {
		t.Fatal(err)
	}
	started := make(chan error, 1)
	go func() { started <- a.Start(context.Background()) }()
	<-r.entered
	if a.State() != app.Starting {
		t.Fatal(a.State())
	}
	ready := make(chan error, 1)
	go func() { ready <- a.Ready(context.Background()) }()
	select {
	case err := <-ready:
		if err == nil {
			t.Fatal("starting app ready")
		}
	case <-time.After(time.Second):
		t.Fatal("readiness blocked by startup mutex")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-started; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	select {
	case <-r.closed:
	default:
		t.Fatal("partially started resource not closed")
	}
	if a.State() != app.Stopped {
		t.Fatal(a.State())
	}
}
func TestRepeatedLifecycleAndConcurrentStop(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	before := runtime.NumGoroutine()
	for i := 0; i < 30; i++ {
		a, err := app.New(config.Config{Address: "127.0.0.1:0", Logger: logger})
		if err != nil {
			t.Fatal(err)
		}
		if err = a.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for j := 0; j < 6; j++ {
			wg.Go(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = a.Ready(ctx)
				if err := a.Shutdown(ctx); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
		if err = a.Start(context.Background()); err == nil {
			t.Fatal("restart permitted")
		}
		listener, err := net.Listen("tcp", a.Address())
		if err != nil {
			t.Fatal("listener leaked", err)
		}
		listener.Close()
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before+4 && time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before+4 {
		t.Fatalf("goroutines grew from %d to %d", before, after)
	}
}
func TestNthResourceFailureCleanupOrder(t *testing.T) {
	for fail := 0; fail < 4; fail++ {
		a := newApp(t)
		events := []string{}
		names := []string{"a", "b", "c", "d"}
		if err := a.Register(moduleFunc(func(reg *app.Registrar) error {
			for i, name := range names {
				if err := reg.Manage(resource{name, &events, i == fail}); err != nil {
					return err
				}
			}
			return nil
		})); err != nil {
			t.Fatal(err)
		}
		if err := a.Start(context.Background()); err == nil {
			t.Fatal("failed resource accepted")
		}
		want := 2 * (fail + 1)
		if len(events) != want {
			t.Fatal(events)
		}
		for i := 0; i <= fail; i++ {
			if events[fail+1+i] != "close:"+names[fail-i] {
				t.Fatal(events)
			}
		}
	}
}
