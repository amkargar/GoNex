# First mapped microservice call

Use a supported patched Go toolchain (minimum 1.27.2). A monolith remains simpler: use `gonex new shop --module YOUR_MODULE` unless you need independent services. This tutorial installs Gonex as a library/tool; do not clone the framework.

## Create the workspace

Install a reviewed CLI version (`@main` is a development snapshot, not a stable release). Replace example.com/acme/commerce with the Go module prefix you own before publishing contracts.

```sh
go install github.com/amkargar/gonex/cmd/gonex@main
gonex new commerce --module example.com/acme/commerce --arch microservices
cd commerce
go mod tidy
go tool gonex create service identity --grpc
go tool gonex create service orders --http
go tool gonex create module users --service identity --transport grpc --version v1
go tool gonex create module orders --service orders --transport http
go tool gonex create client identity --service orders --module orders --version v1 --profile example --rpc UsersService.GetUsersMessage
```

There are seven creation commands, one explicit dependency-preparation step and one run command. Compilation/download time is environment dependent; an independent five-minute onboarding result has not been measured.

The `example` profile is deliberately limited to the descriptor-verified empty-request/string-message contract just generated. It maps the selected RPC response to a plain string. It does not infer a real user/order schema. For your own contract use the default `skeleton` profile, implement your local interface/field/error mapping, then remove its explicit `GONEX_TODO_...` compile-time marker. `doctor` reports its file. `raw` retains the old typed-client-only workflow for advanced manual wiring.

## Make the two user-owned business edits

Replace services/orders/modules/orders/service.go with:

```go
package orders

import "context"

type IdentityReader interface {
    Message(context.Context) (string, error)
}

type Service struct { identity IdentityReader }

func NewService(identity IdentityReader) *Service {
    return &Service{identity: identity}
}

func (s *Service) Message(ctx context.Context) (string, error) {
    return s.identity.Message(ctx)
}
```

This file has no Protobuf or framework imports. Replace only NewModuleService in services/orders/modules/orders/module.go with:

```go
func NewModuleService(r *app.Registrar) (*Service, error) {
    client, err := NewIdentityForModule(r)
    if err != nil { return nil, err }
    return NewService(client), nil
}
```

The generated helper creates one managed pool/config for this target, constructs the exact typed client, registers connection readiness and cleans up on helper failure. It does not choose your domain interface or alter your handwritten factory. Call it once in that factory. For multiple modules/versions, own the service bootstrap explicitly: share a managed rpc.NewClients pool and rpc.NewClientConfig, then use the generated NewIdentityClientWithConfig/NewIdentityV2ClientWithConfig constructors. Do not call multiple convenience helpers for the same target/security profile.

In services/orders/modules/orders/http/routes.go, add `Errors: []int{503, 504},` to the Endpoint literal, so the API explicitly declares provider-unavailable responses. Adapt the generated service_test.go to the constructor you now own (inject a fake IdentityReader); the generator does not rewrite that test. These are application edits; no framework-owned registry needs manual changes.

## Run locally

The explicit root `go mod tidy` above prepares the CLI/framework dependencies. The generated go.work resolves local contracts source for the initial build. New services do not require a fictional contracts release, so no publication is needed for this first run. Service-level go mod tidy requires an actual published contracts version because tidy resolves that module independently of go.work. Prepare the root CLI module for initial local builds. Publication and isolated builds are a separate, explicit step below.

```sh
go tool gonex doctor --dev-insecure
go tool gonex dev --dev-insecure
```

`--dev-insecure` prominently enables **local loopback plaintext only**. The runner refuses it with GONEX_ENV=production and refuses contradictory TLS/insecure settings. No flag is persisted to production templates. Without it, configure server GRPC_TLS_CERT/GRPC_TLS_KEY and client CLIENTS_IDENTITY_CA/SERVER_NAME (plus client certificates when using mTLS). No credentials belong in gonex.yaml and no .env file is loaded automatically.

The console prefixes each service's output and prints configured HTTP/gRPC addresses. Open the Orders address shown by doctor (http://127.0.0.1:8081/orders in this fresh tutorial). The response is `{"message":"users"}`. Configured addresses are not readiness assertions; use /readyz and application logs. Ctrl+C stops all owned services. An unexpected child exit stops its siblings and returns a non-zero result.

`dev --service orders` selects only Orders; it does not silently start Identity. Start its provider separately or it will become unready/return a bounded 503. Readiness is connection readiness, not a promise that every business RPC is healthy. Mutating RPCs are never retried automatically.

## Choose transports and override local addresses

- `create service NAME --http`: HTTP-only listener.
- `create service NAME --grpc`: gRPC-only listener, including the managed health server.
- `create service NAME --http --grpc`: both listeners.
- `create service NAME --transport none`: worker with no listener.
- Existing `create service NAME` keeps its HTTP default. Module adapters and service listeners are distinct; changing a listener later requires an explicit catalog/configuration change, not editing a generated registry.

The caller-editable gonex.yaml owns only local service paths/addresses/declared client edges. .gonex.json owns generator metadata; go.mod owns dependency versions. The existing main.go/config/modules layout is preserved.

Use GONEX_SERVICES_IDENTITY_GRPC_ADDRESS or GONEX_SERVICES_ORDERS_HTTP_ADDRESS to override individual listeners. Provider overrides flow to declared local clients unless CLIENTS_IDENTITY_ADDRESS explicitly overrides the target. Shared HTTP_ADDRESS/GRPC_ADDRESS is accepted only when selecting one service; it is ambiguous for a multi-service run. The local catalog rejects non-loopback addresses; production deployment addresses belong to your normal per-process environment/configuration.

## Publish contracts and deploy independently

New generators do not insert an unpublished contracts requirement. Older templates may contain a v0.1.0 placeholder; it is not a released Gonex contract. Publish contracts as a real Go module using your own repository/subdirectory tags or an immutable Go module proxy. Set the actual published version in each consuming service's go.mod, then explicitly tidy/test/build each service with GOWORK=off. Deployment must not use a local replace or require the workspace.

```sh
# Run inside each consuming service, after publishing the actual contract version.
go get YOUR_CONTRACTS_MODULE@PUBLISHED_VERSION
go mod tidy
go test ./...
go build ./...
```

Set GOWORK=off in the shell for those commands (PowerShell: `$env:GOWORK='off'`). For an already prepared local catalog, `gonex dev --isolated` also uses GOWORK=off. Its builds disable module proxy/network tool downloads, use -mod=readonly and require the chosen toolchain/dependencies to be installed beforehand. Go's normal build cache and local workspace checksum file may be populated; no silent tidy is performed.

CI rehearses this supported publication mechanism by publishing an immutable contracts zip/mod/info fixture to a disposable file proxy, checking its SHA256, then resolving independent service modules without replacements/workspace resolution. That fixture is not a public stable release. The same installed public CLI and mapped tutorial are checked on Linux/Windows.

See [compatibility](compatibility.md), [troubleshooting](troubleshooting.md) and [acceptance](acceptance.md) for exact scope and evidence.
