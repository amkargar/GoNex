# CLI reference

[Project home](../README.md) · [Documentation index](README.md)

The executable is named `gonex`; framework branding is goNex. Run commands from the generated project or workspace root. Positional names come before flags.

## Contents

- [Project creation](#project-creation)
- [Modules and services](#modules-and-services)
- [Adapter changes](#adapter-changes)
- [Run and verify](#run-and-verify)
- [OpenAPI](#openapi)
- [Database migrations](#database-migrations)
- [Protobuf contracts](#protobuf-contracts)
- [Generator safety](#generator-safety)

## Project creation

```sh
gonex new NAME --module IMPORT_PATH [--framework-version VERSION] [--arch monolith|microservices] [--dry-run]
```

The architecture defaults to `monolith`; the framework version defaults to `v0.1.0`. The destination must not exist. Use a resolvable release version or the [explicit local checkout override](getting-started.md#use-an-unpublished-checkout).

## Modules and services

```sh
gonex create module NAME [--transport http|none|grpc|http,grpc] [--http-version vN] [--db] [--service NAME] [--dry-run]
gonex create service NAME [--dry-run]
gonex create client PROVIDER --service CONSUMER --module MODULE [--version v1] [--dry-run]
```

| Option | Behavior |
|---|---|
| `--transport` | Defaults to `http`; `none` creates only the business core |
| `--http-version` | Starts an HTTP module directly in a version package, without legacy routes |
| `--db` | Adds a store interface, GORM adapter, model and explicit migration |
| `--service` | Selects the owning service in a microservices workspace |
| `--version` | Selects the provider's Protobuf API major for a client skeleton |
| `--dry-run` | Validates and prints the proposed plan without changing project files |

`create service` requires a microservices workspace. A client skeleton uses real generated service constructors but leaves domain mapping and injection to the developer. Run `go mod tidy` in each affected module after adding dependencies.

## Adapter changes

```sh
gonex module version NAME --http vN [--policy preserve|replace] [--prefix PATH] [--service NAME] [--dry-run]
gonex module enable NAME --grpc [--service NAME] [--dry-run]
```

Versioning defaults to `preserve` and `/api/vN`. `replace` stops registering legacy routes while retaining their source. Existing version packages are not overwritten. Enabling gRPC preserves handwritten source and requires the supported injectable service factory. See [HTTP versions](versioning.md) and [microservices](microservices.md).

## Run and verify

```sh
gonex run [--service NAME]
gonex build [--service NAME]
gonex check [--service NAME]
gonex --help
gonex --version
```

`run` invokes the Go executable path for the project. `build` builds its packages; `check` runs tests, vet and build. Child command failures preserve their exit codes. A workspace command must identify a service when selection is ambiguous.

## OpenAPI

```sh
gonex openapi generate [--out openapi.json] [--http-version vN] [--service NAME] [--dry-run]
gonex openapi validate [--file openapi.json] [--against BASELINE] [--service NAME]
```

Generation executes the project's initialization/export path without starting listeners, validates the result and writes the output. It exports the legacy document unless `--http-version` selects a registered major. Validation is offline; `--against` additionally runs conservative compatibility checks against a baseline. A generation dry run does not execute project code.

## Database migrations

```sh
gonex db migrate [--service NAME] [--dry-run]
gonex db status [--service NAME] [--dry-run]
```

Configure `DATABASE_URL` and, where needed, `DATABASE_DIALECT` in the executable's environment. Migrations are explicit; serving does not automatically change schemas. Dry runs do not connect to the database. See [database behavior](database-migrations.md).

## Protobuf contracts

```sh
gonex grpc generate [--dry-run]
gonex grpc check
```

Run from a supported project/workspace, service or contracts root. Generation compiles and checks contracts, then invokes pinned official plugins and stages the resulting files. Dry runs omit plugins and writes. `check` compares descriptors to the stored baseline. Neither command substitutes for the release workflow's Buf gate; see [gRPC contracts](grpc.md).

## Generator safety

Generators plan and validate paths and collisions before writes. They reject traversal, symlinks and handwritten-file overwrites, hash-check tool-owned registries and support dry runs. `.gonex.json` records ownership metadata; do not delete it to bypass a conflict.

New projects use a staged rename. Existing projects use an exclusive lock, staging and rollback on ordinary application errors; multi-file updates are not power-loss atomic. After an interrupted process, inspect `.gonex.lock` and proposed changes before recovering. See [getting started](getting-started.md#generator-safety).
