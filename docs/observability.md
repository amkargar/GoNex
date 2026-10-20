# Observability contract

Default request JSON fields are time, level, msg, service, environment, request_id, trace_id, method, path, status and duration_ms; safe error codes are added when available. Path is the registered route pattern, never raw unknown URLs. Every HTTP request receives a server-generated X-Request-ID and a W3C traceparent. Existing unary HTTP→gRPC propagation preserves the trace ID. Request context loggers carry it into services. No streaming trace instrumentation or span-storage/OTLP exporter is claimed.

Metrics are independently opt-in through observability.HTTP/observability/grpc. Each Metrics object retains at most 128 operation labels, including one overflow series; labels longer than 160 bytes, invalid UTF-8 or control characters share overflow. Counters track requests/errors/in-flight work and duration. Eleven cumulative duration buckets include +Inf; finite boundaries are 5, 10, 25, 50, 100, 250, 500, 1000, 2500 and 5000 milliseconds. Snapshots copy their values safely.

`observability/prometheus.Handler(metrics)` is an optional pull exporter using Prometheus text 0.0.4. It has no network client, exporter goroutine or required backend. Mount it deliberately on a protected operational listener/route. An unreachable scrape backend cannot affect API request handling because the application never calls it. Caller-provided readiness callbacks must honor context; scrape cancellation does not forcibly interrupt arbitrary user code. The default application exposes no metrics endpoint.

Run the diagnostic example in examples/observability and follow its README to correlate an injected 503 with the request/trace IDs, error counter, latency histogram and readiness. Business fields such as module are explicit. Production JSON and human-readable development formatting remain configurable; see logging.md. Performance comparisons record core and optional telemetry separately; discard-output telemetry benchmarks exclude storage/network log costs.

References: https://prometheus.io/docs/instrumenting/exposition_formats/ and https://www.w3.org/TR/trace-context/
