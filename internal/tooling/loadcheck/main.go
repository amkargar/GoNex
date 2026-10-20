// Command loadcheck runs a bounded loopback API workload and writes machine-readable evidence.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	apperrors "github.com/amkargar/gonex/errors"
	rpc "github.com/amkargar/gonex/grpc"
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/observability"
	observedgrpc "github.com/amkargar/gonex/observability/grpc"
	"github.com/amkargar/gonex/orm"
	"github.com/gin-gonic/gin"
	googlegrpc "google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"path/filepath"
)

type payload struct {
	Name string `json:"name" validate:"required,min=2,max=80"`
}
type input struct {
	Body payload `body:"true"`
}
type parameter struct {
	ID int `path:"id" validate:"min=1"`
}
type result struct {
	Message string `json:"message"`
}
type module struct {
	database *orm.Database
	rpc      healthpb.HealthClient
}

func (m module) Register(r *app.Registrar) error {
	if m.database != nil {
		if err := transport.Register(r, transport.Endpoint[transport.NoBody, result]{Method: "GET", Path: "/database", OperationID: "database", Status: 200, Errors: []int{503}, Handler: func(ctx context.Context, _ transport.NoBody) (result, error) {
			db, err := m.database.DB()
			if err != nil {
				return result{}, err
			}
			var value string
			err = db.WithContext(ctx).Raw("SELECT value FROM load_rows WHERE id = 1").Scan(&value).Error
			return result{value}, err
		}}); err != nil {
			return err
		}
	}
	if m.rpc != nil {
		if err := transport.Register(r, transport.Endpoint[transport.NoBody, result]{Method: "GET", Path: "/grpc", OperationID: "grpc", Status: 200, Errors: []int{503}, Handler: func(ctx context.Context, _ transport.NoBody) (result, error) {
			reply, err := m.rpc.Check(ctx, &healthpb.HealthCheckRequest{})
			if err != nil {
				return result{}, rpc.FromStatus(err)
			}
			return result{reply.Status.String()}, nil
		}}); err != nil {
			return err
		}
	}

	if err := transport.Register(r, transport.Endpoint[transport.NoBody, result]{Method: "GET", Path: "/get", OperationID: "get", Status: 200, Handler: func(context.Context, transport.NoBody) (result, error) { return result{"hello"}, nil }}); err != nil {
		return err
	}
	if err := transport.Register(r, transport.Endpoint[input, result]{Method: "POST", Path: "/post", OperationID: "post", Status: 201, Handler: func(_ context.Context, in input) (result, error) { return result{in.Body.Name}, nil }}); err != nil {
		return err
	}
	if err := transport.Register(r, transport.Endpoint[parameter, result]{Method: "GET", Path: "/params/:id", OperationID: "params", Status: 200, Handler: func(context.Context, parameter) (result, error) { return result{"found"}, nil }}); err != nil {
		return err
	}
	return transport.Register(r, transport.Endpoint[transport.NoBody, result]{Method: "GET", Path: "/error", OperationID: "error", Status: 200, Errors: []int{404}, Handler: func(context.Context, transport.NoBody) (result, error) {
		return result{}, apperrors.New(apperrors.NotFound, "not found")
	}})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	concurrency := flag.Int("concurrency", 16, "concurrent clients, 1..256")
	limit := flag.Int("requests", 10000, "maximum requests, 1..100000")
	duration := flag.Duration("duration", 5*time.Second, "maximum run duration, up to five minutes")
	scenario := flag.String("scenario", "get", "get, post, params, error, database, grpc")
	telemetry := flag.Bool("telemetry", false, "include completion logs to discard plus trace/metrics cost (excludes disk IO)")
	cpu := flag.String("cpu", "", "optional CPU profile path")
	heap := flag.String("heap", "", "optional heap profile path")
	rate := flag.Int("rate", 0, "optional global requests/second, 0 is unpaced")
	flag.Parse()
	if *rate < 0 || *rate > 100000 {
		return fmt.Errorf("invalid request rate")
	}
	if *concurrency < 1 || *concurrency > 256 || *limit < 1 || *limit > 100000 || *duration <= 0 || *duration > 5*time.Minute {
		return fmt.Errorf("invalid bounded load configuration")
	}
	path := "/" + *scenario
	method := "GET"
	status := 200
	switch *scenario {
	case "get", "database", "grpc":
	case "post":
		method = "POST"
		status = 201
	case "params":
		path = "/params/1"
	case "error":
		status = 404
	default:
		return fmt.Errorf("unknown scenario")
	}
	gin.SetMode(gin.ReleaseMode)
	cfg := config.Config{DisableOpenAPI: true, DisableRequestLogging: !*telemetry, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	if *telemetry {
		cfg.Middleware = []gin.HandlerFunc{observability.HTTP(observability.NewMetrics())}
	}
	a, err := app.New(cfg)
	if err != nil {
		return err
	}
	work := module{}
	if *scenario == "database" {
		dir, err := os.MkdirTemp("", "gonex-load-db-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		db, err := orm.Open(context.Background(), orm.Options{Dialect: "sqlite", DSN: "file:" + filepath.ToSlash(filepath.Join(dir, "load.db")) + "?_pragma=busy_timeout(3000)"})
		if err != nil {
			return err
		}
		defer db.Close(context.Background())
		sql, _ := db.SQL()
		if _, err = sql.Exec("CREATE TABLE load_rows(id INTEGER PRIMARY KEY,value TEXT); INSERT INTO load_rows VALUES (1,'hello')"); err != nil {
			return err
		}
		work.database = db
	}
	if *scenario == "grpc" {
		options := rpc.ServerOptions{Address: "127.0.0.1:0", DevelopmentInsecure: true}
		metrics := observability.NewMetrics()
		if *telemetry {
			options.Interceptors = []googlegrpc.UnaryServerInterceptor{observedgrpc.Server(metrics)}
		}
		server, err := rpc.NewServer(options)
		if err != nil {
			return err
		}
		if err = server.Start(context.Background()); err != nil {
			return err
		}
		defer server.Close(context.Background())
		pool := rpc.NewClients()
		defer pool.Close(context.Background())
		clientOptions := rpc.ClientOptions{Target: server.Address(), DevelopmentInsecure: true}
		if *telemetry {
			clientOptions.Interceptors = []googlegrpc.UnaryClientInterceptor{observedgrpc.Client(metrics)}
		}
		clientConfig, err := rpc.NewClientConfig(clientOptions)
		if err != nil {
			return err
		}
		client, err := rpc.Client(pool, clientConfig, healthpb.NewHealthClient)
		if err != nil {
			return err
		}
		work.rpc = client
	}
	if err = a.Register(work); err != nil {
		return err
	}
	handler, err := a.Handler()
	if err != nil {
		return err
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	transportClient := &http.Transport{MaxIdleConns: *concurrency, MaxIdleConnsPerHost: *concurrency, MaxConnsPerHost: *concurrency}
	defer transportClient.CloseIdleConnections()
	client := http.Client{Transport: transportClient, Timeout: 2 * time.Second}
	if *cpu != "" {
		f, err := os.Create(*cpu)
		if err != nil {
			return err
		}
		defer f.Close()
		if err = pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	beforeG := runtime.NumGoroutine()
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()
	started := time.Now()
	var pace <-chan time.Time
	if *rate > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(*rate))
		defer ticker.Stop()
		pace = ticker.C
	}
	type memorySample struct {
		Seconds    float64 `json:"seconds"`
		Heap       uint64  `json:"heap"`
		Goroutines int     `json:"goroutines"`
	}
	var memoryMu sync.Mutex
	memory := []memorySample{}
	memoryDone := make(chan struct{})
	memoryStopped := make(chan struct{})
	go func() {
		defer close(memoryStopped)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-memoryDone:
				return
			case <-ticker.C:
				var stats runtime.MemStats
				runtime.ReadMemStats(&stats)
				memoryMu.Lock()
				memory = append(memory, memorySample{time.Since(started).Seconds(), stats.HeapAlloc, runtime.NumGoroutine()})
				memoryMu.Unlock()
			}
		}
	}()

	var next atomic.Int64
	var wg sync.WaitGroup
	type sample struct {
		latency time.Duration
		failed  bool
	}
	samples := make(chan sample, *concurrency*2)
	for i := 0; i < *concurrency; i++ {
		wg.Go(func() {
			for {
				if ctx.Err() != nil || next.Add(1) > int64(*limit) {
					return
				}
				if pace != nil {
					select {
					case <-pace:
					case <-ctx.Done():
						return
					}
				}
				var body io.Reader
				if method == "POST" {
					body = bytes.NewBufferString(`{"name":"Ada"}`)
				}
				request, err := http.NewRequestWithContext(ctx, method, server.URL+path, body)
				if err != nil {
					samples <- sample{failed: true}
					continue
				}
				if method == "POST" {
					request.Header.Set("Content-Type", "application/json")
				}
				start := time.Now()
				response, err := client.Do(request)
				failed := err != nil
				if err == nil {
					_, readErr := io.Copy(io.Discard, response.Body)
					response.Body.Close()
					failed = response.StatusCode != status || readErr != nil
				}
				samples <- sample{time.Since(start), failed}
			}
		})
	}
	go func() { wg.Wait(); close(samples) }()
	latencies := make([]float64, 0, *limit)
	failures := 0
	cancelled := 0
	for sample := range samples {
		if sample.failed {
			if ctx.Err() != nil {
				cancelled++
			} else {
				failures++
			}
		}
		latencies = append(latencies, float64(sample.latency.Microseconds())/1000)
	}
	close(memoryDone)
	<-memoryStopped
	elapsed := time.Since(started)
	transportClient.CloseIdleConnections()
	server.Close()
	runtime.GC()
	runtime.ReadMemStats(&after)
	if *heap != "" {
		f, err := os.Create(*heap)
		if err != nil {
			return err
		}
		err = pprof.WriteHeapProfile(f)
		f.Close()
		if err != nil {
			return err
		}
	}
	if len(latencies) == 0 {
		return fmt.Errorf("no samples")
	}
	sort.Float64s(latencies)
	quantile := func(q float64) float64 { return latencies[int(float64(len(latencies)-1)*q)] }
	report := map[string]any{"scenario": *scenario, "rate": *rate, "memory_series": memory, "telemetry": *telemetry, "samples": len(latencies), "errors": failures, "deadline_cancellations": cancelled, "error_rate": float64(failures) / float64(len(latencies)), "duration_seconds": elapsed.Seconds(), "throughput_rps": float64(len(latencies)) / elapsed.Seconds(), "p50_ms": quantile(.5), "p95_ms": quantile(.95), "p99_ms": quantile(.99), "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "concurrency": *concurrency, "heap_before": before.HeapAlloc, "heap_after_gc": after.HeapAlloc, "goroutines_before": beforeG, "goroutines_after": runtime.NumGoroutine(), "commit": os.Getenv("GITHUB_SHA")}
	if err = json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("workload errors: %d", failures)
	}
	return nil
}
