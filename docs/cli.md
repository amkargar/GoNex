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

The architecture defaults to `monolith`. The framework version defaults to the installed CLI’s resolved module version, including Go pseudo-versions. A CLI built locally must receive an explicit valid `--framework-version`. The destination must not exist.

Install with `go install github.com/amkargar/gonex/cmd/gonex@main` for development, or pin a reviewed commit/published tag. Generated apps include a tool directive: inside the app use `go tool gonex` for the commands below. Microservices workspaces also have a root tool module; run `go mod tidy` there after creation.

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
gonex build [--os OS] [--arch ARCH] [--out FILE] [--service NAME]
gonex check [--service NAME]
gonex --help
gonex --version
```

`run` invokes the Go executable path for the project; the application displays its bound address and writes structured lifecycle/request logs. See [logging](logging.md). `build` writes the application executable to `dist/<os>-<arch>/`; `check` runs tests, vet and package compilation. Child command failures preserve their exit codes. A workspace command must identify a service when selection is ambiguous.

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

## Upgrade dependency and generator metadata

After selecting a reviewed target, run `go get -tool github.com/amkargar/gonex/cmd/gonex@TARGET` in the application. Resolve its canonical module version with `go list -m github.com/amkargar/gonex`, then run `go tool gonex upgrade --framework-version CANONICAL_VERSION --dry-run` and apply without --dry-run after reviewing it. Finish with go mod tidy, tests and contract comparisons.

Upgrade changes only go.mod tool/require metadata and .gonex.json. It does not migrate handwritten code, replace registries or change SQL schemas. For a microservices workspace run it at the tool root and in each service module deliberately. Restore reviewed go.mod/go.sum and binaries for a rollback; that does not undo migrations. The public consumer CI rehearses a real source-version upgrade and verifies a handwritten marker survives.

## Microservices workflow

create service NAME accepts --http, --grpc, both, or --transport none; no flags retains HTTP defaults. create client accepts --profile skeleton|example|raw and explicit --rpc Service.Method for the limited example contract. Module/client --version vN selects RPC contracts; HTTP versioning remains separate.

doctor [--service NAME] [--dev-insecure] [--json] reports configuration and mapping diagnostics without secrets. dev [--service NAME] [--dev-insecure] [--generate] [--isolated] builds cached dependencies and supervises direct processes. Child failure stops siblings. Generation is explicit; no silent tidy/tool download occurs. Follow [the full tutorial](microservices-dx/quickstart.md).


## Command help

Use `gonex -h`, `gonex --help` or `gonex help` for the grouped command overview.
Every command supports contextual help, including `gonex new -h`,
`gonex create module -h`, `gonex build --help` and `gonex help create module`.
Help exits successfully without checking the project or generating files.
Terminal output uses restrained colors on a supported terminal and plain text
when redirected, when `TERM=dumb` or when `NO_COLOR` is set.

## Build an executable

`gonex build` invokes `go build -o <file> .` for the selected application,
rather than only compiling the package tree.

```sh
go tool gonex build
go tool gonex build --os windows --arch amd64
go tool gonex build --os windows --arch arm64
go tool gonex build --os linux --arch amd64
go tool gonex build --os linux --arch arm64
go tool gonex build --os linux --arch amd64 --out ./release/server
go tool gonex build --service orders --os linux --arch arm64
```

Targets map to Go's `GOOS` and `GOARCH`. Their defaults are the environment
values, falling back to the machine running the CLI. Target pairs are validated
against `go tool dist list`. Other Go build environment settings, including
`CGO_ENABLED`, remain under the caller's control; dependencies using CGO can
require a target C toolchain when cross-compiling.

The default output is `dist/<os>-<arch>/<app>`, with `.exe` added for Windows.
`--out` selects an exact file path; relative paths resolve from the selected
application/service root. Success reports the target and executable path.
A run command is shown only when the target OS and architecture match the host;
the Windows run command is formatted for PowerShell.

Compilation errors remain visible and retain the Go subprocess exit code.
`gonex check` continues to test, vet and compile all application packages.
