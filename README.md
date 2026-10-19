# Gonex

A Go framework with explicit modules, Gin HTTP adapters and transport-independent business services.

Requires Go 1.27.1. Implementation proceeds in verified, separately committed phases. See [architecture](docs/architecture.md), [API decisions](docs/api-decisions.md) and [phase reports](docs/phases/0.md).

Status: baseline established. Runtime, OpenAPI, persistence, generators and gRPC are not yet implemented.

```sh
go test ./...
go vet ./...
go build ./...
```
