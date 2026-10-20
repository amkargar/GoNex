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
	transport "github.com/amkargar/gonex/http"
	"github.com/amkargar/gonex/observability"
	"github.com/gin-gonic/gin"
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
type module struct{}

func (module) Register(r *app.Registrar) error {
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
	scenario := flag.String("scenario", "get", "get, post, params, error")
	telemetry := flag.Bool("telemetry", false, "include completion logs to discard plus trace/metrics cost (excludes disk IO)")
	cpu := flag.String("cpu", "", "optional CPU profile path")
	heap := flag.String("heap", "", "optional heap profile path")
	flag.Parse()
	if *concurrency < 1 || *concurrency > 256 || *limit < 1 || *limit > 100000 || *duration <= 0 || *duration > 5*time.Minute {
		return fmt.Errorf("invalid bounded load configuration")
	}
	path := "/" + *scenario
	method := "GET"
	status := 200
	switch *scenario {
	case "get":
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
	if err = a.Register(module{}); err != nil {
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
	report := map[string]any{"scenario": *scenario, "telemetry": *telemetry, "samples": len(latencies), "errors": failures, "deadline_cancellations": cancelled, "error_rate": float64(failures) / float64(len(latencies)), "duration_seconds": elapsed.Seconds(), "throughput_rps": float64(len(latencies)) / elapsed.Seconds(), "p50_ms": quantile(.5), "p95_ms": quantile(.95), "p99_ms": quantile(.99), "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "cpus": runtime.NumCPU(), "concurrency": *concurrency, "heap_before": before.HeapAlloc, "heap_after_gc": after.HeapAlloc, "goroutines_before": beforeG, "goroutines_after": runtime.NumGoroutine(), "commit": os.Getenv("GITHUB_SHA")}
	if err = json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("workload errors: %d", failures)
	}
	return nil
}
