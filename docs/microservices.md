# Microservices workspaces

[Project home](../README.md) · [Documentation index](README.md)

## Contents

- [Generate a workspace](#generate-a-workspace)
- [Understand module boundaries](#understand-module-boundaries)
- [Select services and ports](#select-services-and-ports)
- [Add or enable adapters](#add-or-enable-adapters)
- [Complete client mapping](#complete-client-mapping)
- [Configure client targets](#configure-client-targets)
- [Configure transport security](#configure-transport-security)
- [Verify service behavior](#verify-service-behavior)
- [Run the completed example](#run-the-completed-example)

## Generate a workspace

[Install a versioned CLI](getting-started.md#install-the-cli) first. Each service resolves its own dependencies. The workspace root tracks the CLI as a Go tool; generated contracts are workspace-local until you publish your contracts module.

```sh
gonex new commerce --module example.com/acme/commerce --arch microservices
cd commerce
go mod tidy
go tool gonex create service identity
go tool gonex create service orders
go tool gonex create module users --service identity --transport grpc
go tool gonex create module orders --service orders --transport http
go tool gonex create client identity --service orders --module orders --version v1
go tool gonex grpc generate
go -C services/identity mod tidy
go -C services/orders mod tidy
go tool gonex check --service identity
go tool gonex check --service orders
```

## Understand module boundaries

Each service has its own go.mod, main.go, config and explicit module registry. Contracts have a separate module and versioned Go packages. go.work is only local development convenience; production requires published, resolvable framework/contracts versions. Templates contain no replace directives or absolute paths. The test harness supplies explicit isolated replacements and verifies each module with GOWORK=off.

## Select services and ports

New services get distinct default loopback HTTP/gRPC ports (8081/9091, 8082/9092, ...), with HTTP_ADDRESS/GRPC_ADDRESS overrides. `gonex run/build/check --service NAME` select a deployable from the workspace root; ambiguity is an error. OpenAPI, database and version commands also accept --service. Each service exports its own DATABASE_URL and owns its migrations. No generated repository reads another service's database.

## Add or enable adapters

gRPC modules expose the starter service's real Message use case as Get<Module>Message with distinct versioned request/response messages. grpc-only profiles have no HTTP adapter. http,grpc profiles share one service instance and one managed server per process. All contracts and official generated stubs are staged before any project write. Dry runs create no project files. `gonex module enable users --grpc --service identity` preserves handwritten module/service/HTTP files and adds the transport; old template modules need the documented NewModuleService/injectable Service factory before automatic enable is safe.

## Complete client mapping

Client generation reads the selected provider/version descriptors and uses the exact generated service constructors. The adapter file contains real typed clients plus a TODO to implement the consumer's small local business interface. It does not invent domain mappings or wire dependencies into handwritten business code. Add methods mapping plain values to/from contract messages, create the adapter with a shared managed Clients pool, and inject it into your service. See integration/rpc for a completed Identity/Orders mapping, and internal/cli/microservices_test.go for a generated-workspace mapping and executable failure test.

## Configure client targets

Generated client settings use typed Address/TLS/Timeout fields; LoadIdentityClient reads CLIENTS_IDENTITY_ADDRESS, CA, SERVER_NAME, CERT, KEY and INSECURE. Address defaults to the provider's development loopback endpoint. This is explicit static configuration, not distributed service discovery.

## Configure transport security

Production gRPC needs GRPC_TLS_CERT and GRPC_TLS_KEY, optionally GRPC_CLIENT_CA for mTLS. Clients need matching trust/identity settings. Local plaintext requires explicit GONEX_GRPC_INSECURE=1 for servers and CLIENTS_IDENTITY_INSECURE=1 for clients. Export GONEX_DISABLE_HTTP=1 for a purely gRPC provider. There is no automatic dotenv loading and no generated production secrets.

## Verify service behavior

Library tests exercise real TLS/mTLS. Generated-workspace end-to-end tests use explicit development mode, run independently built provider/consumer binaries, verify an HTTP→gRPC response, terminate the provider and require a bounded dependency-unavailable response. A new client skeleton does not provide that business behavior until its mapping and wiring are completed.

## Run the completed example

Run the completed [Identity/Orders example](../examples/microservices/README.md) for an interactive two-process demo, including tracing, readiness, explicit plaintext development flags and TLS/mTLS configuration. After generating new dependencies, run go mod tidy separately in each service. Export selected HTTP contracts with `gonex openapi generate --service orders --http-version v1 --out orders-v1.json` when that version is registered.
