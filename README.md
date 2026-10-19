# Gonex

A Go framework with explicit modules, Gin HTTP adapters and transport-independent business services.

Requires Go 1.27.1. Implementation proceeds in verified, separately committed phases. See [architecture](docs/architecture.md), [API decisions](docs/api-decisions.md) and [phase reports](docs/phases/0.md).

Status: Phases 0–6 implemented, including optional TLS/mTLS gRPC, managed clients and reproducible versioned Protobuf contracts. Microservices generation and release hardening are next.

```sh
go test ./...
go vet ./...
go build ./...
go run ./examples/basic
```

The example serves `GET /healthz` → `{"status":"ok"}`, `GET /users/1` → `{"id":"1","name":"Ada","email":"ada@example.com"}`, `POST /users` → 201, `GET /demo` → `{"message":"Gonex"}` and `GET /openapi.json` on `127.0.0.1:8080`. Set `HTTP_ADDRESS` to override; use Ctrl+C to stop. The app owns one HTTP server and closes resources in reverse startup order. Registration freezes when startup begins; stopped applications cannot restart. Configuration defaults: read 10s, write 30s, idle 60s, shutdown 10s.

See [HTTP contracts](docs/http-openapi.md) and the per-phase reports for verification and limitations.

Build the CLI with `go build -o gonex.exe ./cmd/gonex`; see [getting started](docs/getting-started.md). Each implementation phase has its own commit and report under `docs/phases/`.
