# Gonex

A Go framework with explicit modules, Gin HTTP adapters and transport-independent business services.

Requires Go 1.27.1. Implementation proceeds in verified, separately committed phases. See [architecture](docs/architecture.md), [API decisions](docs/api-decisions.md) and [phase reports](docs/phases/0.md).

Status: Phases 0–1 implemented: explicit modules, Gin routes, managed resources and bounded HTTP shutdown. OpenAPI, persistence, generators and gRPC are not yet implemented.

```sh
go test ./...
go vet ./...
go build ./...
go run ./examples/basic
```

The example serves `GET /healthz` → `{"status":"ok"}`, `GET /users` → `[{"id":"1","name":"Ada"}]` and `GET /demo` → `{"message":"Gonex"}` on `127.0.0.1:8080`. Set `HTTP_ADDRESS` to override; use Ctrl+C to stop. The app owns one HTTP server and closes resources in reverse startup order. Registration freezes when startup begins; stopped applications cannot restart. Configuration defaults: read 10s, write 30s, idle 60s, shutdown 10s.
