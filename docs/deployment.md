# Deploying a Gonex API

Build your application with the patched Go baseline, pin go.mod/go.sum, run tests and export/review OpenAPI before promotion. Start with the independently installable [Orders sample](../examples/production/README.md) and its Docker/Compose rehearsal; it consumes a real public commit without local replacements.

Run migrations as an explicit deployment step before starting the new binary. Prefer additive/backward-compatible schema changes and verify backups/restores. Run the API under a process/container supervisor, send SIGTERM for drain, and allow more time than ShutdownTimeout for termination. Set GONEX_ENV=production through generated/application configuration for structured JSON logs. Environment loading belongs to the executable; .env is not loaded automatically.

Terminate public HTTPS at a verified reverse proxy/platform and forward registered API paths without accidental prefix stripping. Bind the Go listener to loopback behind a local proxy, or the intended container interface behind a controlled network. Use verified database TLS and explicit credential management; the private plaintext Compose network is only a disposable integration environment.

Use /healthz for liveness and /readyz for traffic admission. Remove an unready instance from traffic; diagnose its request/trace IDs and dependency readiness. Do not expose /metrics, management APIs or schema documents publicly without a deliberate policy. Request contexts must carry deadlines into database/RPC calls. Business operations must define idempotency before adding retries.

Rollout acceptance includes authn/authz/validation/CRUD E2E, dependency outage/recovery, shutdown under active requests and deployment-hardware load. Retain binary/image digests, Go/dependency versions, environment and results. A container build alone is not a deployed production sign-off. See production-checklist.md, troubleshooting.md and production-readiness/final-report.md.
