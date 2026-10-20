<p align="center">
  <img src="docs/assets/gonex-header.svg" alt="goNex — modular Go framework" width="760">
</p>

---

[![Go 1.27.1](https://img.shields.io/badge/Go-1.27.1-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev/)
[![Dependencies: 10 direct](https://img.shields.io/badge/dependencies-10%20direct-informational?style=flat)](docs/architecture.md#dependencies)
[![CI status](https://github.com/amkargar/gonex/actions/workflows/test.yml/badge.svg)](https://github.com/amkargar/gonex/actions/workflows/test.yml)
[![License: BSD-3-Clause](https://img.shields.io/badge/license-BSD--3--Clause-blue?style=flat)](LICENSE)

goNex is a modular Go framework for typed HTTP APIs and explicit service boundaries. It combines Gin routing, typed request/response contracts and explicit module registration. Keep business services in plain Go, then add HTTP, persistence or gRPC adapters where you need them. The application owns startup, readiness and graceful shutdown; the CLI scaffolds projects without scanning source at runtime.

**Development status:** phases 0–8 have implementation commits. Local checks pass; hosted CI, Linux race tests and live PostgreSQL verification remain pending. No release has been published. See the [verification record](docs/phases/8.md) and [release policy](docs/release.md).

## Contents

- [Features](#features)
- [Quick start](#quick-start)
- [Create an application](#create-an-application)
- [Project structure](#project-structure)
- [Documentation](#documentation)
- [Examples](#examples)
- [Configuration](#configuration)
- [Development and verification](#development-and-verification)
- [Compatibility and scope](#compatibility-and-scope)
- [License](#license)

## Features

| Area | What goNex provides |
|---|---|
| Application | Explicit modules, atomic registration, managed resources and graceful shutdown |
| HTTP | Gin adapters, typed binding, strict JSON, validation and request limits |
| API contracts | OpenAPI 3.1, versioned HTTP adapters and baseline compatibility checks |
| Persistence | Optional GORM, SQLite/PostgreSQL adapters and explicit checksum-verified SQL migrations |
| RPC | Optional TLS/mTLS gRPC, managed clients, deadlines and versioned Protobuf contracts |
| CLI | Safe project/module scaffolding, dry runs, independent services and reproducible generators |
| Operations | Dependency readiness, trace propagation and in-process request/error/latency metrics, structured logging and startup URLs |
| Security | Application-owned authentication, authorization and explicit CORS policies |

Business services depend on small local interfaces. Gin, GORM, gRPC and generated messages stay in adapters. See the [architecture and dependencies](docs/architecture.md).

## Quick start

Requires **Go 1.27.1**. Install Gonex as a library/tool dependency; work in your own application, without cloning this framework:

```sh
go install github.com/amkargar/gonex/cmd/gonex@main
gonex new shop --module example.com/acme/shop
cd shop
go mod tidy
go tool gonex create module users
go tool gonex check
go tool gonex run
```

`@main` installs a development snapshot; choose a reviewed commit or a real published tag when pinning a particular build. The generated project pins the resolved CLI/framework module version. Ensure Go's binary directory is on PATH for the initial `gonex` command.

## Create an application

Develop JSON APIs in generated modules. `schemas.go` defines request/response types and validation, handlers call plain Go services, and routes declare HTTP methods and OpenAPI contracts. This gives a DRF-inspired API workflow with explicit Go composition. There is no application HTML template or CSS layer.

`GET /users` initially returns `{"message":"users"}`; extend the handwritten module files for your domain. Database CRUD is not generated automatically. Persistence, API versioning, gRPC and microservices are optional.

The running terminal shows the actual server URL, OpenAPI URL and graceful-stop instructions. Request logs include status, duration and request ID. Development uses readable output; production can use JSON. See [logging](docs/logging.md).

See [getting started](docs/getting-started.md), [CLI](docs/cli.md) and the [official documentation](https://amkargar.github.io/GoNex-website/docs/main/).

## Project structure

The business core and transport adapters have separate responsibilities:

```text
modules/
└── users/
    ├── module.go            # registration and service construction
    ├── service.go           # business logic and local interfaces
    └── http/
        ├── handlers.go      # transport-to-service calls
        ├── schemas.go       # request and response DTOs
        ├── routes.go        # typed endpoint registration
        └── openapi.go       # contract metadata
```

Persistence and gRPC add adapters around the same core. HTTP versions live in independent adapter packages; microservices use separate Go modules and share a versioned contracts module. See [modules and lifecycle](docs/modules.md) and [microservices](docs/microservices.md).

## Documentation

The [documentation index](docs/README.md) provides a reading path and guide directory. Guides include a table of contents and links back to the index.

| Build | Operate | Maintain |
|---|---|---|
| [Getting started](docs/getting-started.md) | [Operations and readiness](docs/operations.md) | [Architecture](docs/architecture.md) |
| [Modules and lifecycle](docs/modules.md) | [Security configuration](docs/security.md) | [Compatibility and upgrades](docs/upgrade.md) |
| [HTTP and OpenAPI](docs/http-openapi.md) | [Databases and migrations](docs/database-migrations.md) | [Release policy](docs/release.md) |
| [HTTP versions](docs/versioning.md) | [Optional gRPC](docs/grpc.md) | [Benchmark methodology](docs/benchmarks/README.md) |
| [CLI reference](docs/cli.md) | [Microservices](docs/microservices.md) | [Changelog](CHANGELOG.md) |

## Examples

| Example | Purpose | Start here |
|---|---|---|
| Basic application | Typed users API, memory/SQLite repository, health and optional metrics | [Getting started](examples/basic/main.go) |
| Identity and Orders | Independent HTTP → gRPC services, readiness and TLS configuration | [Service example](examples/microservices/README.md) |
| TLS integration fixture | Generated contracts, domain mapping and bounded provider failure | [Integration source](integration/rpc/rpc_test.go) |

Generated client adapters contain explicit business-mapping TODOs. Complete their mapping and dependency wiring before using them as an application integration.

## Configuration

| Setting | Default or behavior |
|---|---|
| HTTP address | `127.0.0.1:8080`; executable configuration can override it |
| HTTP read / write / idle | 10s / 30s / 60s |
| Shutdown budget | 10s |
| Typed request body limit | 1 MiB; configurable per endpoint |
| OpenAPI | Enabled; set `DisableOpenAPI` to remove public documents |
| Documentation page | Disabled; opt in with `DocsEnabled` |
| gRPC | No listener unless explicitly managed; TLS required by default |
| RPC client timeout | 5s; earlier caller deadlines take precedence |
| Logging | Readable development output, JSON in production, request IDs; [configuration](docs/logging.md) |
| Metrics endpoint | No framework endpoint; basic example opts in with `GONEX_EXPOSE_METRICS=1` |

Environment loading belongs to the executable. The examples and generated applications document their own variables; there is no automatic `.env` loading. Review [security defaults](docs/security.md) before deployment.

## Development and verification

```sh
go test ./...
go vet ./...
go build ./...
go run ./cmd/gonex grpc check
```

Nested modules have independent checks; see [release verification](docs/release.md). The workflows cover [Go checks](.github/workflows/test.yml), [contract checks](.github/workflows/contracts.yml) and [release artifacts](.github/workflows/release.yml). The CI badge reports hosted workflow state; local verification is recorded separately.

Benchmarks compare paired Gin/GORM workloads on the same machine. Read the [methodology and recorded results](docs/benchmarks/README.md) for scope and regression budgets.

Implementation reports: [0](docs/phases/0.md) · [1](docs/phases/1.md) · [2](docs/phases/2.md) · [3](docs/phases/3.md) · [4](docs/phases/4.md) · [5](docs/phases/5.md) · [6](docs/phases/6.md) · [7](docs/phases/7.md) · [8](docs/phases/8.md).

## Compatibility and scope

Pin framework and contracts versions. HTTP adapters and Protobuf packages have independent API majors; compare exported contracts against reviewed baselines before upgrading. Generators preserve handwritten files and reject conflicts in tool-owned registries. See [upgrade policy](docs/upgrade.md).

Current limits include static service targets, cooperative cancellation, conservative compatibility checks, manual business mappings and in-process metrics. Streaming trace propagation, an external tracing exporter, brokers, an admin panel and advanced discovery are outside the current implementation.

## License

goNex is licensed under the **BSD 3-Clause License (`BSD-3-Clause`)**. See [LICENSE](LICENSE) for the full terms.
