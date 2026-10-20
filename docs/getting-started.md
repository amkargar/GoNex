# Getting started: build an API

[Project home](../README.md) · [Documentation index](README.md)

Gonex is an API framework installed as a Go dependency. Create your own application with the CLI, generate modules, and implement JSON endpoints inside that application. No framework checkout, Git setup, HTML templates or CSS are required.

## Requirements

Use Go 1.27.2 and access to Go modules. Gonex is in development; no release tag is assumed. Install a development snapshot with `@main`, or replace it with a reviewed commit or an actual published tag. Generated applications pin the resolved version used by the installed CLI.

## Install the CLI

```sh
go install github.com/amkargar/gonex/cmd/gonex@main
gonex --version
```

Ensure `GOBIN` (or `GOPATH/bin` when unset) is on PATH. This bootstrap command installs a development tool; your application's framework dependency is recorded separately in its go.mod.

## Create a project

```sh
gonex new shop --module example.com/acme/shop
cd shop
go mod tidy
go tool gonex create module users
go tool gonex check
go tool gonex run
```

Use your application's module path. The generated go.mod contains the framework dependency and a `tool github.com/amkargar/gonex/cmd/gonex` directive. Continue with `go tool gonex` inside the app so the project selects its CLI version. The tool is not linked into your application binary.

`GET http://127.0.0.1:8080/users` initially returns `{"message":"users"}`. This is an editable starter endpoint, not an automatic database CRUD API. Inspect `/openapi.json` for its contract and `/healthz` for liveness.

## Develop inside your application

- `modules/users/service.go`: plain Go business behavior.
- `modules/users/http/schemas.go`: request/response DTOs and validation tags.
- `modules/users/http/handlers.go`: map HTTP input to services and services to responses.
- `modules/users/http/routes.go`: typed endpoints, methods and response statuses.
- `modules/users/http/openapi.go`: contract descriptions.
- `modules/users/module.go`: service construction and adapter registration.

These roles resemble serializers, views and routing in DRF, with explicit Go types and composition. Gonex does not provide Django-style page rendering, an admin site or automatic ModelViewSets. Use a separate frontend to consume the JSON API.

Generated registries and `.gonex.json` are tool-owned. The other files are yours. If a new adapter needs a different service interface, change your module's registration code deliberately; do not alter a generated registry to bypass ownership checks.

## Server output and logs

The terminal shows the actual listening URL after successful startup, the OpenAPI URL when enabled, the environment, version and Ctrl+C instructions. Requests produce one completion log with method, registered route, status, elapsed milliseconds and request ID. Business code can retrieve the request logger from its context; see [logging](logging.md).

The generated configuration reads `APP_NAME`, `HTTP_ADDRESS`, `GONEX_ENV`, `GONEX_LOG_LEVEL`, `GONEX_LOG_FORMAT` and `GONEX_LOG_COLOR`. Development defaults to readable output; `GONEX_ENV=production` defaults to JSON. `.env.example` documents the variables; `.env` is not loaded automatically.

## Existing Go applications

```sh
go get github.com/amkargar/gonex@main
```

Register modules and start Gonex from your executable. A CLI-generated application is the recommended starting point, but the public library also supports manual composition. Installing a dependency does not replace your existing HTTP server automatically. Run `go mod tidy` after adding imports.

## Optional persistence, versions and RPC

```sh
go tool gonex create module messages --db
go tool gonex create module catalog --http-version v1
```

Database modules require explicit configuration and migration. HTTP versioning, gRPC and microservices remain opt-in. See [CLI](cli.md), [database](database-migrations.md), [HTTP contracts](http-openapi.md) and [versions](versioning.md).

## Generator safety

Use `--dry-run` to inspect a plan. Generators reject existing destinations, invalid names, traversal, symlinks and overwritten handwritten files. Tool-owned registries are hash-checked. Writes use staging and a lock, with rollback on ordinary errors; multi-file updates are not power-loss atomic. Inspect an interrupted `.gonex.lock` before recovery.

A CLI built from a local checkout has no published module version. For framework development only, pass a valid `--framework-version VERSION` explicitly and use your intentional local replacement. Ordinary application development does not need this workflow.

## Independent services

Follow [the mapped microservice tutorial](microservices-dx/quickstart.md). Installation stays library-based, business edits are explicit and dev/doctor follow dependency preparation. Monolith remains the default.
