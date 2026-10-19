package tests

import (
	"context"
	"errors"
	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	rpc "github.com/amkargar/gonex/grpc"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/observability"
	observedgrpc "github.com/amkargar/gonex/observability/grpc"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	pb "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/status"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type tracedService struct {
	pb.UnimplementedTestServiceServer
}

func (tracedService) UnaryCall(ctx context.Context, in *pb.SimpleRequest) (*pb.SimpleResponse, error) {
	if string(in.GetPayload().GetBody()) == "panic" {
		panic("secret panic value")
	}
	select {
	case <-time.After(20 * time.Millisecond):
	case <-ctx.Done():
		return nil, rpc.ToStatus(ctx.Err())
	}
	return &pb.SimpleResponse{Payload: &pb.Payload{Body: []byte(observability.Traceparent(ctx))}}, nil
}

type traceResponse struct {
	Trace string `json:"trace"`
}

func TestTracePropagationMetricsReadiness(t *testing.T) {
	metrics := observability.NewMetrics()
	server, e := rpc.NewServer(rpc.ServerOptions{Address: "127.0.0.1:0", DevelopmentInsecure: true, Interceptors: []grpc.UnaryServerInterceptor{observedgrpc.Server(metrics)}})
	if e != nil {
		t.Fatal(e)
	}
	if e = server.Register(func(s *grpc.Server) error { pb.RegisterTestServiceServer(s, tracedService{}); return nil }); e != nil {
		t.Fatal(e)
	}
	pool := rpc.NewClients()
	a, e := app.New(config.Config{Address: "127.0.0.1:0", Middleware: []gin.HandlerFunc{observability.HTTP(metrics)}})
	if e != nil {
		t.Fatal(e)
	}
	var client pb.TestServiceClient
	metrics.SetReadiness(a.Ready)
	var dependency atomic.Bool
	dependency.Store(true)
	if e = a.Register(moduleFunc(func(r *app.Registrar) error {
		for _, resource := range []app.Resource{server, pool, metrics} {
			if e := r.Manage(resource); e != nil {
				return e
			}
		}
		if e := r.Check("dependency", func(context.Context) error {
			if !dependency.Load() {
				return errors.New("secret dependency credentials")
			}
			return nil
		}); e != nil {
			return e
		}
		return transport.Register(r, transport.Endpoint[transport.NoBody, traceResponse]{Method: "GET", Path: "/trace", OperationID: "trace", Status: 200, Handler: func(ctx context.Context, _ transport.NoBody) (traceResponse, error) {
			response, e := client.UnaryCall(ctx, &pb.SimpleRequest{})
			if e != nil {
				return traceResponse{}, e
			}
			return traceResponse{Trace: string(response.Payload.Body)}, nil
		}})
	})); e != nil {
		t.Fatal(e)
	}
	if e = a.Ready(context.Background()); e == nil {
		t.Fatal("created app reported ready")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if e = a.Start(ctx); e != nil {
		t.Fatal(e)
	}
	defer a.Shutdown(context.Background())
	options, e := rpc.NewClientConfig(rpc.ClientOptions{Target: server.Address(), DevelopmentInsecure: true, Interceptors: []grpc.UnaryClientInterceptor{observedgrpc.Client(metrics)}})
	if e != nil {
		t.Fatal(e)
	}
	if e = pool.Ready(ctx, options); e != nil {
		t.Fatal(e)
	}
	client, e = rpc.Client(pool, options, pb.NewTestServiceClient)
	if e != nil {
		t.Fatal(e)
	}
	h, e := a.Handler()
	if e != nil {
		t.Fatal(e)
	}
	incoming := "00-11111111111111111111111111111111-2222222222222222-01"
	request := httptest.NewRequest("GET", "/trace", nil)
	request.Header.Set("traceparent", incoming)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "11111111111111111111111111111111") || !observability.ValidTraceparent(w.Header().Get("traceparent")) {
		t.Fatal(w.Code, w.Body, w.Header())
	}
	if w.Header().Get("traceparent") == incoming {
		t.Fatal("HTTP span was not changed")
	}
	snapshot := metrics.Snapshot()
	measure := snapshot.Operations["HTTP GET /trace"]
	if !snapshot.Ready || measure.Requests != 1 || measure.Errors != 0 || measure.InFlight != 0 || measure.TotalNanoseconds <= 0 {
		t.Fatal(snapshot)
	}
	rpcOperations := 0
	for name, measure := range snapshot.Operations {
		if strings.HasPrefix(name, "gRPC ") {
			rpcOperations++
			if measure.Requests != 1 || measure.Errors != 0 {
				t.Fatal(name, measure)
			}
		}
	}
	if rpcOperations != 2 {
		t.Fatal(snapshot)
	}
	if _, e = client.UnaryCall(ctx, &pb.SimpleRequest{Payload: &pb.Payload{Body: []byte("panic")}}); status.Code(e) != codes.Internal || strings.Contains(e.Error(), "secret") {
		t.Fatal(e)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	dependency.Store(false)
	if metrics.Snapshot().Ready {
		t.Fatal("failed dependency reported ready in metrics")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body)
	}
	if e = a.Shutdown(ctx); e != nil {
		t.Fatal(e)
	}
	if metrics.Snapshot().Ready {
		t.Fatal("stopped metrics stayed ready")
	}
}

func TestUnknownHTTPLabelsAreBounded(t *testing.T) {
	var metrics observability.Metrics
	a, err := app.New(config.Config{Middleware: []gin.HandlerFunc{observability.HTTP(&metrics)}})
	if err != nil {
		t.Fatal(err)
	}
	h, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"CUSTOMONE", "CUSTOMTWO"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, "/"+method, nil))
	}
	snapshot := metrics.Snapshot()
	if len(snapshot.Operations) != 1 || snapshot.Operations["HTTP OTHER unmatched"].Requests != 2 {
		t.Fatal(snapshot)
	}
}

func TestReadinessConcurrentWithShutdown(t *testing.T) {
	a := newApp(t)
	metrics := observability.NewMetrics()
	if err := a.Register(moduleFunc(func(r *app.Registrar) error { return r.Manage(metrics) })); err != nil {
		t.Fatal(err)
	}
	if err := a.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Go(func() {
			for j := 0; j < 100; j++ {
				_ = a.Ready(context.Background())
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	workers.Wait()
	if err := a.Ready(ctx); err == nil {
		t.Fatal("stopped app reported ready")
	}
}

func TestInflightHTTPDrainsBeforeResourcesClose(t *testing.T) {
	a := newApp(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer func() {
		releaseOnce.Do(func() { close(release) })
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = a.Shutdown(ctx)
	}()
	if e := a.Register(moduleFunc(func(r *app.Registrar) error {
		return r.Handle("GET", "/slow", func(c *gin.Context) { close(entered); <-release; c.JSON(200, gin.H{"done": true}) })
	})); e != nil {
		t.Fatal(e)
	}
	if e := a.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	response := make(chan error, 1)
	go func() {
		client := http.Client{Timeout: 2 * time.Second}
		res, e := client.Get("http://" + a.Address() + "/slow")
		if e == nil {
			_, e = io.ReadAll(res.Body)
			res.Body.Close()
			if res.StatusCode != 200 {
				e = errors.New("request failed")
			}
		}
		response <- e
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- a.Shutdown(ctx) }()
	select {
	case e := <-stopped:
		t.Fatal("shutdown did not drain", e)
	case <-time.After(20 * time.Millisecond):
	}
	if e := a.Ready(ctx); e == nil {
		t.Fatal("stopping app reported ready")
	}
	releaseOnce.Do(func() { close(release) })
	if e := <-response; e != nil {
		t.Fatal(e)
	}
	if e := <-stopped; e != nil {
		t.Fatal(e)
	}
}

type failingSource struct {
	errors chan error
	closed atomic.Bool
}

func (s *failingSource) Start(context.Context) error { return nil }
func (s *failingSource) Close(context.Context) error { s.closed.Store(true); return nil }
func (s *failingSource) Errors() <-chan error        { return s.errors }
func TestManagedAsynchronousFailureStopsApp(t *testing.T) {
	a := newApp(t)
	resource := &failingSource{errors: make(chan error, 1)}
	if e := a.Register(moduleFunc(func(r *app.Registrar) error { return r.Manage(resource) })); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	resource.errors <- errors.New("managed failure")
	select {
	case e := <-done:
		if e == nil || !resource.closed.Load() || a.State() != app.Stopped {
			t.Fatal(e, a.State())
		}
	case <-ctx.Done():
		t.Fatal("managed failure hung")
	}
}
