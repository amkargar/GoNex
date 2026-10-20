# Lifecycle and outage contract

States are Created, Registered, Starting, Running, Stopping and Stopped. Registration is allowed only before startup. A stopped instance cannot restart. Shutdown is idempotent; concurrent callers wait for the first shutdown or their own context deadline. Startup callbacks run without the state mutex, so readiness/State do not wait behind an arbitrary Start callback. Shutdown during Starting cancels startup and waits within the caller's budget for rollback.

Partially started resources are included in reverse-order cleanup. Listener ownership is explicit, including the interval before net/http has registered the listener in Serve; an immediate shutdown cannot leave that listening socket unowned. Serve-error delivery is nonblocking so a simultaneous resource failure and HTTP stop do not strand a goroutine. Readiness requires Running and cooperative resource/check success.

HTTP shutdown stops accepting traffic, drains active requests within the configured budget, then forces HTTP close when necessary. Resource close follows in reverse order. gRPC has graceful/forced stop and call deadlines. Arbitrary application Start/Close/Ready callbacks and custom log writers cannot be forcibly killed safely: they must honor contexts, and must not recursively drive application lifecycle from a logger callback. A callback violating this contract remains an application blocker, not a supported forced-termination guarantee.

Tests cover Nth-resource failure, shutdown while starting, repeated start/stop with socket rebinding, concurrent shutdown/readiness, HTTP drain and RPC force-stop. Leak checks use a bounded goroutine tolerance and repeated loops, not an assertion that long-running deployments can never leak.

For an outage: inspect request/trace IDs, readiness and dependency status; keep retries bounded and avoid retrying non-idempotent writes automatically. Restore the network/database and verify readiness before returning traffic. An open SQL pool can reconnect; a closed resource needs a new application instance. Binary rollback requires schema compatibility and does not reverse migrations. See database-reliability.md and troubleshooting.md.
