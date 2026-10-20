# Troubleshooting and recovery

| Symptom | Check and recovery |
| --- | --- |
| CLI missing | Inspect go env GOBIN/GOPATH and PATH; install an exact public version, then use the project-local tool. |
| New app cannot resolve Gonex | Check the installed CLI's module version and Go proxy access; never substitute an invented release tag. |
| Startup fails | No ready event is expected before successful binding. Check safe startup errors, address collision and required configuration; fix the cause and create a new app instance. |
| Ready is 503 | Inspect managed database/RPC readiness with bounded contexts; distinguish a closed pool from a transient connection outage. |
| Request is 401/403 | Check actual verifier/policy middleware, credential format and role requirements; OpenAPI metadata does not enforce security. Never log tokens. |
| Migration rejected | Check immutable filenames/checksums/history and concurrent runners. Do not edit applied SQL to force success; restore/reconcile explicitly and ship a forward fix. |
| Database returns unavailable | Correlate request/trace IDs, inspect network/pool limits and restore connectivity. Verify readiness before traffic returns; do not blindly retry writes. |
| Shutdown exceeds budget | Find callbacks ignoring context, blocked custom log writers or handlers not propagating cancellation. Framework HTTP/RPC close paths are bounded; arbitrary user code is cooperative. |
| Metrics missing/overflow | Metrics/exporter are opt-in. Use fixed route/RPC labels, inspect the 128-operation budget, and protect the scrape route. |
| High latency or retained heap | Reproduce the same workload/hardware, compare core versus telemetry, examine CPU/heap profiles and paced soak trends; do not compare unrelated hosts as a regression proof. |

See resilience.md, database-reliability.md, observability.md and deployment.md. Record the incident, affected version, recovery steps and residual risk; a passing initial test suite does not replace incident history.
