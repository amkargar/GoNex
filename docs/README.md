# goNex documentation

[Project home](../README.md) · [Getting started](getting-started.md) · [CLI reference](cli.md) · [Changelog](../CHANGELOG.md)

Build typed APIs with explicit modules, keep business logic independent of transports, and add persistence or RPC when the application needs them.

## Contents

- [Start here](#start-here)
- [Build an application](#build-an-application)
- [Operate a service](#operate-a-service)
- [Maintain and upgrade](#maintain-and-upgrade)
- [Examples](#examples)
- [Implementation record](#implementation-record)

## Start here

1. [Run the basic example](getting-started.md#run-the-basic-example) directly from the checkout.
2. [Create your project](getting-started.md#create-a-project) and configure a local dependency override until releases are published.
3. Read [modules and lifecycle](modules.md) to understand registration and resource ownership.
4. Define [typed HTTP contracts](http-openapi.md), then choose the integrations you need.

**Requirements:** the repository targets Go 1.27.1. Downloads need network access or a populated module cache. There is no published framework release yet; hosted CI and live PostgreSQL verification are still pending. See [release and support policy](release.md) for the current verification boundary.

## Build an application

| Guide | What you will learn |
|---|---|
| [Getting started](getting-started.md) | Run examples, build the CLI and create a project |
| [CLI reference](cli.md) | Commands, flags, profiles and safe generator behavior |
| [Modules and lifecycle](modules.md) | Register features, inject services and own resources |
| [Typed HTTP and OpenAPI](http-openapi.md) | Bind requests, validate DTOs and export contracts |
| [HTTP versions](versioning.md) | Preserve legacy routes and add independent API adapters |
| [Databases and migrations](database-migrations.md) | Use GORM adapters and apply explicit SQL migrations |
| [Optional gRPC](grpc.md) | Configure TLS, clients and reproducible Protobuf contracts |
| [Microservices workspaces](microservices.md) | Generate independent modules and wire service clients |

## Operate a service

| Guide | What you will learn |
|---|---|
| [Operations](operations.md) | Readiness, trace propagation, metrics and shutdown |
| [Security configuration](security.md) | Authentication, authorization, CORS, limits and redaction |
| [Release and support policy](release.md) | Verification gates, artifacts and publication prerequisites |

## Maintain and upgrade

| Guide | What you will learn |
|---|---|
| [Architecture](architecture.md) | Dependency direction, package boundaries and direct dependencies |
| [API decisions](api-decisions.md) | Constructor, registration, lifecycle and contract guarantees |
| [Compatibility and upgrades](upgrade.md) | Module versions, contract baselines and template changes |
| [Benchmarks](benchmarks/README.md) | Workloads, recorded measurements and regression budgets |
| [Changelog](../CHANGELOG.md) | Changes in the unpublished implementation |

## Examples

- [Basic application source](../examples/basic/main.go): typed HTTP, an optional database and opt-in metrics.
- [Identity and Orders walkthrough](../examples/microservices/README.md): two independently runnable services with HTTP → gRPC mapping.
- [TLS integration fixture](../integration/rpc/rpc_test.go): generated contracts, plain business interfaces and failure scenarios.

Generated client skeletons require application-specific mapping and dependency wiring. The completed examples show those boundaries explicitly.

## Implementation record

Phase reports are historical engineering records; use the guides above for current usage.

| Phase | Report |
|---|---|
| 0 | [Baseline and decisions](phases/0.md) |
| 1 | [Application core and HTTP lifecycle](phases/1.md) |
| 2 | [Typed HTTP and OpenAPI](phases/2.md) |
| 3 | [GORM and migrations](phases/3.md) |
| 4 | [CLI and project scaffolding](phases/4.md) |
| 5 | [HTTP version adapters](phases/5.md) |
| 6 | [Secure gRPC and contracts](phases/6.md) |
| 7 | [Independent services and clients](phases/7.md) |
| 8 | [Operational hardening and release gates](phases/8.md) |

The intended project license is BSD 3-Clause. The owner will add the license file; see the [license status](../README.md#license).
