# Gonex

A Go framework with explicit modules, Gin HTTP adapters and transport-independent business services.

Requires Go 1.27.1. Implementation proceeds in verified, separately committed phases. See [architecture](docs/architecture.md), [API decisions](docs/api-decisions.md) and [phase reports](docs/phases/0.md).

Status: Phases 0–8 implemented in separate commits. Includes typed HTTP/OpenAPI, optional GORM and TLS/mTLS gRPC, reproducible contracts, safe monolith/microservice scaffolding, readiness, tracing, metrics and security middleware. Local checks pass; hosted CI, Linux race and live PostgreSQL gates remain unverified. This checkout is not a published release.

```sh
go test ./...
go vet ./...
go build ./...
go run ./examples/basic
```

The example serves `GET /healthz` → `{"status":"ok"}`, `GET /users/1` → `{"id":"1","name":"Ada","email":"ada@example.com"}`, `POST /users` → 201, `GET /demo` → `{"message":"Gonex"}` and `GET /openapi.json` on `127.0.0.1:8080`. Set `HTTP_ADDRESS` to override; use Ctrl+C to stop. The app owns one HTTP server and closes resources in reverse startup order. Registration freezes when startup begins; stopped applications cannot restart. Configuration defaults: read 10s, write 30s, idle 60s, shutdown 10s.

See [HTTP contracts](docs/http-openapi.md) and the per-phase reports for verification and limitations.

Build the CLI with `go build -o gonex.exe ./cmd/gonex`; see [getting started](docs/getting-started.md). Each implementation phase has its own commit and report under `docs/phases/`.

Guides: [modules](docs/modules.md), [database](docs/database-migrations.md), [gRPC](docs/grpc.md), [HTTP versions](docs/versioning.md), [microservices](docs/microservices.md), [operations](docs/operations.md), [security](docs/security.md), [upgrades](docs/upgrade.md), [release policy](docs/release.md) and [benchmarks](docs/benchmarks/README.md). The [two-service example](examples/microservices/README.md) demonstrates HTTP → gRPC with explicit TLS/development configuration.

Phase reports: [0](docs/phases/0.md), [1](docs/phases/1.md), [2](docs/phases/2.md), [3](docs/phases/3.md), [4](docs/phases/4.md), [5](docs/phases/5.md), [6](docs/phases/6.md), [7](docs/phases/7.md), [8](docs/phases/8.md).
