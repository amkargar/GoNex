# Optional gRPC

[Project home](../README.md) · [Documentation index](README.md)

## Contents

- [Opt in to gRPC](#opt-in-to-grpc)
- [Register a server](#register-a-server)
- [Configure TLS and mTLS](#configure-tls-and-mtls)
- [Reuse managed clients](#reuse-managed-clients)
- [Set deadlines and drain requests](#set-deadlines-and-drain-requests)
- [Generate versioned contracts](#generate-versioned-contracts)
- [Check compatibility](#check-compatibility)
- [Run the TLS integration fixture](#run-the-tls-integration-fixture)
- [Recovery and observability](#recovery-and-observability)
- [Run Buf checks](#run-buf-checks)

## Opt in to gRPC

Import `github.com/amkargar/gonex/grpc` only in transport/bootstrap code. HTTP-only modules do not import it or start gRPC listeners. `config.Config{DisableHTTP:true}` permits a gRPC-only process. Manage servers and client pools with Registrar.Manage so failed startup/shutdown releases them. Resources exposing Errors() also report asynchronous failures to App.Run.

## Register a server

`grpc.NewServer(ServerOptions{Address:...,TLS:...})` constructs without binding. Supply generated registration callbacks through Server.Register, then let the app Start/Close it. The built-in standard gRPC health service reports serving status. Application services must not import gRPC or generated messages; map at server/client boundaries using ToStatus/FromStatus. Unknown errors are redacted.

## Configure TLS and mTLS

TLS is required by default. Load a server certificate and trust roots using crypto/tls and crypto/x509. mTLS uses RequireAndVerifyClientCert plus ClientCAs; the client supplies its certificate. Client InsecureSkipVerify is rejected. Only explicit DevelopmentInsecure permits plaintext for local development. Keep caller-provided TLS certificate/key material immutable after configuration.

## Reuse managed clients

Create one immutable ClientConfig per target/security/transport setup with NewClientConfig. Reuse that config with a shared Clients manager to reuse a connection concurrently. Different immutable configs never share a connection, even when targets are equal; this avoids credential/identity/dialer mixing. `grpc.Client(pool, config, generated.NewServiceClient)` returns a typed generated client. Creation is lazy; `pool.Ready(ctx,config)` separately proves connectivity and requires a bounded caller context. Registry resolves explicit service names to configs. No write retry policy is installed; configured retry policies are disabled.

## Set deadlines and drain requests

Default unary and stream RPC timeouts are five seconds, configurable per server/client; shorter incoming contexts win. Stream timeouts bound the whole stream lifetime. Handlers must honor their contexts. Server.Close uses graceful stop, then force-stop if the supplied context expires. App.Run supplies its configured shutdown timeout.

## Generate versioned contracts

Contracts are a separate Go module at contracts/. All packages and Go paths are versioned. `gonex grpc generate` compiles with pinned Buf protocompile v0.14.1 and invokes official protoc-gen-go v1.36.10 / protoc-gen-go-grpc v1.6.0. This Go-only compiler is the one used by Buf; protoc installation is unnecessary. `--dry-run` compiles/checks without plugins or writes. Generated files are staged; sources and handwritten adapters are preserved. Network/cache access is needed to obtain pinned tools on first use.

## Check compatibility

schema.binpb is the generated descriptor set. baseline.binpb is the explicitly frozen initial release contract. `gonex grpc check` rejects removed/changed files, messages, fields, enum values and RPC signatures, allowing additions. Generation enforces this baseline too. This conservative checker may require a new major for option-only changes; CI also runs Buf FILE compatibility against the same descriptor baseline. Update baselines only during a reviewed release, never to suppress a failed compatibility gate. buf.yaml and buf.gen.yaml support a standard external Buf toolchain as well.

## Run the TLS integration fixture

`integration/rpc` is an isolated executable-contract test module with explicit relative local replacements for this checkout. Run `go -C contracts test ./...` and `go -C integration/rpc test ./...`. Its Identity provider and Orders consumer use generated GetUser messages over real TLS, with plain Go business interfaces. These test-only replacements are not production template dependencies.

## Recovery and observability

Unary and stream recovery returns redacted Internal errors. `grpc.ServerTLS` and `grpc.ClientTLS` load verified certificate configurations with redacted file errors. The optional observability/grpc unary interceptors propagate traceparent and collect fixed-method metrics; see [operations](operations.md). Application readiness can check managed server lifecycle and explicitly registered downstream client connectivity.

## Run Buf checks

Install the pinned official Buf binary with `go run ./internal/tooling/bufinstall .tools`; it verifies the downloaded release checksum. Run `.tools/buf lint contracts/proto` and `.tools/buf breaking contracts/proto --against contracts/baseline.binpb` (use buf.exe on Windows). CI runs these in addition to internal checks. The [runnable Identity/Orders example](../examples/microservices/README.md) includes HTTP-to-gRPC mapping and TLS configuration.
