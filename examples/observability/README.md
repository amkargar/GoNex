# Diagnose a dependency failure

Run `go run ./examples/observability` from the framework checkout. This is a maintainer diagnostic example; application developers install the package/CLI through the main getting-started guide. No Prometheus server or collector is required.

1. Request `curl -i http://127.0.0.1:8080/work`; record X-Request-ID and traceparent and find the corresponding terminal completion log.
2. Create an empty `dependency.down` file in the working directory (`touch dependency.down`, or `New-Item dependency.down` in PowerShell).
3. Request /work again: expect 503. Match its request/trace IDs with the `dependency unavailable` business event and completion event. /readyz must also return 503.
4. Request /metrics. Observe `gonex_errors_total` for `HTTP GET /work`, cumulative duration buckets and `gonex_ready 0`. Labels contain registered operations, not identities or URLs.
5. Delete dependency.down and request /readyz and /work again; both recover. Stop with Ctrl+C.

The metrics route is loopback-only for this example. Protect it if deploying on a public interface. The exported format can be scraped by Prometheus; the example requires no external backend. An HTTP→gRPC propagation regression is exercised in tests/operations_test.go, separately from this local dependency fixture.
