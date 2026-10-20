// Package prometheus provides an opt-in, pull-based Prometheus 0.0.4 exporter.
// It adds no background workers or external dependency to an application.
package prometheus

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/amkargar/gonex/observability"
)

func Handler(metrics *observability.Metrics) (http.Handler, error) {
	if metrics == nil {
		return nil, fmt.Errorf("metrics required")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(405)
			return
		}
		snapshot := metrics.SnapshotContext(r.Context())
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == "HEAD" {
			return
		}
		var out strings.Builder
		out.WriteString("# HELP gonex_requests_total Completed requests.\n# TYPE gonex_requests_total counter\n# HELP gonex_errors_total Failed requests.\n# TYPE gonex_errors_total counter\n# HELP gonex_in_flight Active requests.\n# TYPE gonex_in_flight gauge\n# HELP gonex_request_duration_seconds Request duration in seconds.\n# TYPE gonex_request_duration_seconds histogram\n# HELP gonex_ready Application readiness.\n# TYPE gonex_ready gauge\n")
		ready := 0
		if snapshot.Ready {
			ready = 1
		}
		fmt.Fprintf(&out, "gonex_ready %d\n", ready)
		names := make([]string, 0, len(snapshot.Operations))
		for name := range snapshot.Operations {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			value := snapshot.Operations[name]
			label := strconv.Quote(name)
			fmt.Fprintf(&out, "gonex_requests_total{operation=%s} %d\ngonex_errors_total{operation=%s} %d\ngonex_in_flight{operation=%s} %d\n", label, value.Requests, label, value.Errors, label, value.InFlight)
			for i, bound := range observability.LatencyBounds() {
				fmt.Fprintf(&out, "gonex_request_duration_seconds_bucket{operation=%s,le=%q} %d\n", label, strconv.FormatFloat(bound.Seconds(), 'g', -1, 64), value.DurationBuckets[i])
			}
			fmt.Fprintf(&out, "gonex_request_duration_seconds_bucket{operation=%s,le=\"+Inf\"} %d\ngonex_request_duration_seconds_sum{operation=%s} %g\ngonex_request_duration_seconds_count{operation=%s} %d\n", label, value.Requests, label, float64(value.TotalNanoseconds)/1e9, label, value.Requests)
		}
		_, _ = w.Write([]byte(out.String()))
	}), nil
}
