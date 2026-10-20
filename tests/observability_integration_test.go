package tests

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/amkargar/gonex/app"
	"github.com/amkargar/gonex/config"
	"github.com/amkargar/gonex/observability"
	exporter "github.com/amkargar/gonex/observability/prometheus"
	"github.com/gin-gonic/gin"
)

func TestHistogramCardinalityAndExporter(t *testing.T) {
	m := observability.NewMetrics()
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		i := i
		wg.Go(func() { done := m.Begin(fmt.Sprintf("fixed route %d", i)); done(i%2 == 0); done(false) })
	}
	wg.Wait()
	snapshot := m.Snapshot()
	if len(snapshot.Operations) > observability.MaxOperations {
		t.Fatal("cardinality unbounded")
	}
	var requests int64
	for _, v := range snapshot.Operations {
		requests += v.Requests
		if v.InFlight != 0 || v.DurationBuckets[10] != v.Requests {
			t.Fatal(v)
		}
		for i := 1; i < len(v.DurationBuckets); i++ {
			if v.DurationBuckets[i] < v.DurationBuckets[i-1] {
				t.Fatal("noncumulative histogram")
			}
		}
	}
	if requests != 1000 {
		t.Fatal(requests)
	}
	h, err := exporter.Handler(m)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(w.Header().Get("Content-Type"), "version=0.0.4") || !strings.Contains(w.Body.String(), `le="+Inf"`) {
		t.Fatal(w)
	}
	if _, err = exporter.Handler(nil); err == nil {
		t.Fatal("nil collector accepted")
	}
}
func TestDefaultLogTraceCorrelation(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	m := observability.NewMetrics()
	a, err := app.New(config.Config{Name: "orders", Environment: "production", Logger: logger, Middleware: []gin.HandlerFunc{observability.HTTP(m)}})
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Register(moduleFunc(func(r *app.Registrar) error {
		return r.Handle("GET", "/failure/:id", func(c *gin.Context) {
			observability.Logger(c.Request.Context(), nil).ErrorContext(c.Request.Context(), "dependency unavailable", "module", "orders")
			c.Status(503)
		})
	})); err != nil {
		t.Fatal(err)
	}
	h, _ := a.Handler()
	w := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/failure/private?token=secret", nil)
	h.ServeHTTP(w, request)
	trace := w.Header().Get("traceparent")
	if !observability.ValidTraceparent(trace) {
		t.Fatal(trace)
	}
	records := logRecords(t, &output)
	if len(records) != 2 {
		t.Fatal(records)
	}
	for _, record := range records {
		if record["trace_id"] != strings.Split(trace, "-")[1] || record["request_id"] != w.Header().Get("X-Request-ID") || record["service"] != "orders" {
			t.Fatal(record)
		}
	}
	if strings.Contains(output.String(), "private") || strings.Contains(output.String(), "secret") {
		t.Fatal("sensitive URL leaked")
	}
	measurement := m.SnapshotContext(context.Background()).Operations["HTTP GET /failure/:id"]
	if measurement.Errors != 1 {
		t.Fatal(measurement)
	}
}
