# Operations

[Project home](../README.md) · [Documentation index](README.md)

## Contents

- [Health and readiness](#health-and-readiness)
- [Propagate trace context](#propagate-trace-context)
- [Collect metrics](#collect-metrics)
- [Expose example metrics](#expose-example-metrics)
- [Drain and stop services](#drain-and-stop-services)
- [Verify operational behavior](#verify-operational-behavior)

## Structured logs

Lifecycle and request logs are enabled by default. See [server output and logging](logging.md) for readable development output, JSON production output, request IDs and configuration. Trace middleware and metrics remain independently opt-in.

## Health and readiness

`/healthz` reports that HTTP is serving. `/readyz` returns 200 only while the application is running and its managed readiness resources and named checks succeed; otherwise it returns a redacted 503. `Registrar.Check(name, func(context.Context) error)` registers an application-specific check. Checks share a one-second budget and must honor context. The GORM resource pings its pool; the gRPC server checks its transport lifecycle. Check downstream RPC availability explicitly with `Clients.Ready`, rather than assuming a constructed client is connected.

## Propagate trace context

`observability.HTTP(metrics)` is optional Gin middleware installed through `config.Config.Middleware`. Default request logging already establishes a validated traceparent and correlated log trace ID. This middleware reuses that context, or creates it when request logging is disabled; incoming version-00 headers preserve the trace ID and get a new HTTP span ID. Invalid/duplicate headers start a fresh trace. `observability.Traceparent(ctx)` reads the current value. Optional `observability/grpc.Client` and `Server` unary interceptors propagate it across services; credentials and arbitrary incoming metadata are not forwarded. Streaming trace propagation and an OTLP exporter are not implemented.

## Collect metrics

`observability.NewMetrics()` is a managed resource. Counters track completed requests, errors, in-flight work and cumulative elapsed nanoseconds by fixed route/RPC name. Unknown HTTP URLs share an unmatched label; raw URLs, user IDs and secrets are excluded. `SnapshotContext` copies measurements for a caller-owned exporter. Call `SetReadiness(application.Ready)` to include dependency readiness; without that callback, readiness represents the metrics resource's lifecycle only. Snapshots include cumulative latency histograms and a fixed operation-cardinality budget. The optional observability/prometheus pull handler exports Prometheus text format without an external backend requirement; no persistent metrics store or scrape server is started automatically. See [observability](observability.md).

## Expose example metrics

The basic example enables tracing/counters and exposes its JSON metrics route only with `GONEX_EXPOSE_METRICS=1`. Protect that route at your deployment boundary if enabled. There is no exporter endpoint by default in framework applications.

## Drain and stop services

Shutdown first stops accepting HTTP traffic and drains active requests within the shutdown budget, then closes managed resources in reverse order. gRPC uses graceful stop with a forced stop when its context expires. Signal handling belongs to the executable. HTTP deadline middleware cancels request contexts; handlers and dependency calls must honor them. A handler that ignores context cannot be forcibly interrupted safely.

## Verify operational behavior

Run `go test ./tests ./grpc` for readiness, HTTP drain, asynchronous resource failure, RPC graceful/forced stop, stream deadlines, trace propagation and error-redaction regressions. See the [runnable service example](../examples/microservices/README.md) for HTTP-to-gRPC requests and provider failure.
