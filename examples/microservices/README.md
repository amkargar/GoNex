# Identity and Orders

[Project home](../../README.md) · [Documentation index](../../docs/README.md)

Two independent Go modules share the versioned contracts module in this checkout. The relative replacements are explicit example conveniences; generated production service modules have no replacements.

## Contents

- [Start Identity](#start-identity)
- [Start Orders](#start-orders)
- [Call the API](#call-the-api)
- [Enable TLS and mTLS](#enable-tls-and-mtls)
- [Understand the service boundaries](#understand-the-service-boundaries)
- [Verify independent modules](#verify-independent-modules)

## Start Identity

In one PowerShell console, from this directory:

```powershell
$env:GONEX_GRPC_INSECURE='1' # explicit local development
go -C services/identity run .
```

## Start Orders

In a second console:

```powershell
$env:CLIENTS_IDENTITY_INSECURE='1' # explicit local development
go -C services/orders run .
```

## Call the API

`GET http://127.0.0.1:8082/orders/users/1` returns `{"id":"1","name":"Ada","email":"ada@example.com"}`. `/readyz` checks the Identity connection. `/openapi.json` documents the typed HTTP route. Unknown users return 404; stopping Identity produces a bounded 503/504. Ctrl+C stops either service gracefully. traceparent propagates from HTTP to gRPC, retaining the trace ID and creating new server span IDs.

## Enable TLS and mTLS

For TLS, set GRPC_TLS_CERT/GRPC_TLS_KEY on Identity and CLIENTS_IDENTITY_CA/CLIENTS_IDENTITY_SERVER_NAME on Orders. For mTLS also set GRPC_CLIENT_CA and CLIENTS_IDENTITY_CERT/CLIENTS_IDENTITY_KEY. Leave both development insecure flags unset in that configuration. HTTP_ADDRESS, GRPC_ADDRESS and CLIENTS_IDENTITY_ADDRESS override the loopback defaults.

## Understand the service boundaries

The service.go files import context only (plus domain errors on Identity). Protobuf mapping lives in transport adapters. Orders has no Identity database access.

## Verify independent modules

Each service is verified independently with `GOWORK=off go test ./...` in its own directory; there is no required root workspace for deployment. Publish resolvable module versions and replace the checkout-only dependency overrides when deploying this example.
