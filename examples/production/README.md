# Independent Orders API rehearsal

This is a standalone, API-only consumer of a pinned public Gonex commit. Its go.mod has no replace or go.work dependency on this checkout. You can copy this directory outside the framework repository and run it independently. It is a production-like verification sample, not a declaration that every production-readiness gate is complete.

For a new application, follow the public CLI quick start: https://amkargar.github.io/GoNex-website/docs/main/quick-start/ . This advanced sample owns its custom bootstrap/module wiring and uses ordinary Go build/run commands rather than pretending it is an unmodified CLI scaffold.

## Features and boundaries

- Identity adapter: two distinct caller-supplied opaque bearer tokens. Readers can list/get; administrators can create/update/delete. This is role-based application authentication, not a password store, OAuth server, JWT issuer or per-user tenant-isolation scheme.
- Orders: typed schemas/handlers/routes, service interfaces independent of HTTP, GORM repository and explicit SQL migration. Offset pagination is limited to 50 rows and offset 10000; optional status filtering accepts pending/completed. Updates are last-writer-wins, not optimistic concurrency.
- Operational behavior: bounded request contexts, health/readiness, JSON production logs, request/trace IDs, managed pool shutdown and opt-in metrics. OpenAPI describes bearer security, but actual enforcement lives in middleware.
- PostgreSQL is the deployment database. SQLite is used for fast deterministic local tests. No templates or frontend assets are served.

## Run the standalone service

Use Go 1.27.2 or a later supported patched 1.27 release. From this directory:

```sh
go mod download
go test ./...
go build -trimpath -o api .
```

Export DATABASE_DIALECT=postgres and DATABASE_URL for an empty application database. Generate separate random secrets for GONEX_ADMIN_TOKEN and GONEX_READER_TOKEN (at least 32 bytes each), for example with `openssl rand -hex 24`, and provide them through your secret manager/process environment. No default credentials are generated and .env is not automatically loaded.

```sh
./api migrate
GONEX_ENV=production HTTP_ADDRESS=127.0.0.1:8080 ./api
```

On Windows build api.exe and set the environment variables in PowerShell before running it. Serving does not run migrations. An existing database needs a reviewed migration/backup policy.

Use Authorization: Bearer YOUR_TOKEN for /api/orders. POST accepts `{"item":"Notebook","quantity":2}`; PUT /api/orders/{id} accepts the same fields plus `"status":"completed"`. DELETE returns 204. GET /api/orders?limit=20&offset=0&status=pending returns items, limit and offset. /healthz and /readyz are public operational probes. Decide at your deployment boundary whether /openapi.json should be exposed; GONEX_DISABLE_OPENAPI=1 disables it.

## Docker Compose rehearsal

Export POSTGRES_PASSWORD (use URL-safe random hex), GONEX_ADMIN_TOKEN and GONEX_READER_TOKEN, then:

```sh
docker compose -f deployments/compose.yaml build api
docker compose -f deployments/compose.yaml up -d
python3 scripts/e2e.py
```

The migration container must succeed before API startup. The API binds only to host loopback, runs as nonroot with a read-only root filesystem and drops capabilities. PostgreSQL is on the private Compose network with a named disposable volume. Plain database transport is limited to this isolated rehearsal; production database transport and public HTTP require verified TLS at the appropriate boundaries.

To rehearse an outage, stop the postgres service and verify /readyz returns 503. Start it, wait for readiness to recover, then run `python3 scripts/e2e.py --check-recovery`; it checks a retained row from the first run before exercising CRUD again. CI automates this drill. `docker compose -f deployments/compose.yaml down -v` destroys rehearsal data; use only for disposable environments.

## Tests and upgrade rehearsal

Service unit tests use a fake Store without HTTP/database dependencies. Integration tests exercise real network HTTP and SQLite; GONEX_PRODUCTION_TEST_DSN enables isolated-schema PostgreSQL tests. CI sets GONEX_REQUIRE_POSTGRES=1 so PostgreSQL cannot skip. Tests cover 401/403, validation, pagination/filtering, create/get/update/delete, OpenAPI, DB failure and readiness.

A source-commit upgrade is rehearsed separately by the public consumer smoke tool. A tagged-release upgrade and rollback cannot be certified before a reviewed candidate exists. For a real rehearsal: save the old go.mod/go.sum and binary digest, update to a reviewed candidate, run tests/contract comparisons, migrate only with backward-compatible changes, run E2E/load probes, then restore the old binary and verify it against the same schema. Never assume binary rollback undoes SQL migrations. Record the actual environment, commands and results in the final readiness report.
