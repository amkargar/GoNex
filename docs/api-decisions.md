# API decisions

[Project home](../README.md) · [Documentation index](README.md)

These decisions began as the phase 0 contract and now describe the implemented application API. Historical results remain in the [phase reports](README.md#implementation-record).

## Contents

- [Module and toolchain](#module-and-toolchain)
- [Construction and registration](#construction-and-registration)
- [Routing and contracts](#routing-and-contracts)
- [Resource ownership](#resource-ownership)
- [Lifecycle guarantees](#lifecycle-guarantees)

## Module and toolchain

The Go module is `github.com/amkargar/gonex`; the product name is goNex. The repository baseline is Go 1.27.1. Exact dependency versions are pinned in [go.mod](../go.mod). See [release policy](release.md) for configured and verified platforms.

## Construction and registration

- `gonex.New(config.Config) (*app.App, error)` validates settings without binding ports.
- `Module.Register(*Registrar) error` declares a module's routes and resources.
- `App.Register(...Module) error` registers explicitly in argument order, atomically per call.
- Registration freezes at startup. A failed registration call leaves the application unchanged.
- `Registrar.Handle(method, path string, handlers ...gin.HandlerFunc) error` is a raw Gin escape hatch. Use typed HTTP registration for public contracts.

## Routing and contracts

Route identity is the uppercase method plus full path. Duplicate and incompatible wildcard routes fail before serving. Runtime health/readiness and enabled document paths reject conflicting registrations.

`http.Register` builds a typed endpoint contract. Request origins, supported validation constraints and schema projection are checked together. Contracts detach mutable metadata at registration. Raw Gin routes are not included in OpenAPI.

HTTP versions are explicit adapter packages. gRPC remains optional, with versioned Protobuf packages and independently managed servers/clients. See [HTTP contracts](http-openapi.md) and [gRPC](grpc.md).

## Resource ownership

`Registrar.Manage(Resource) error` owns resources implementing `Start(context.Context) error` and `Close(context.Context) error`. Registration order determines startup order; close runs in reverse. Dependencies register before their consumers. An optional Errors channel reports asynchronous failure to App.Run.

`Registrar.Check(name, func(context.Context) error)` adds a named dependency readiness check. Resources may also implement `Ready(context.Context) error`. Application callbacks must honor context cancellation.

## Lifecycle guarantees

| API | Behavior |
|---|---|
| `App.Start(ctx)` | Starts managed resources and binds the HTTP listener synchronously |
| `App.Run(ctx)` | Starts, waits for cancellation/failure and shuts down within the configured budget |
| `App.Shutdown(ctx)` | Drains HTTP and closes resources; repeated calls are safe |
| `App.Ready(ctx)` | Checks running state and registered dependency readiness |

States are created, registered, starting, running, stopping and stopped. A stopped instance cannot restart. `App.Logger() *slog.Logger` returns the application-scoped logger. Configuration includes logging, address, middleware, exposure switches and read/write/idle/shutdown timeouts. Zero durations use defaults; negative values fail. See [modules](modules.md) and [operations](operations.md).
