# HTTP versions

[Project home](../README.md) · [Documentation index](README.md)

## Contents

- [Add an HTTP version](#add-an-http-version)
- [Preserve or replace legacy routes](#preserve-or-replace-legacy-routes)
- [Choose a prefix](#choose-a-prefix)
- [Export independent contracts](#export-independent-contracts)
- [Start with a versioned module](#start-with-a-versioned-module)
- [Handle custom adapter layouts](#handle-custom-adapter-layouts)

## Add an HTTP version

```sh
gonex module version users --http v1 --policy preserve --dry-run
gonex module version users --http v1
gonex module version users --http v2
```

## Preserve or replace legacy routes

The default preserve policy retains existing handlers and URLs. Adapter sources are copied into independent http/v1 and http/v2 packages; generated registration calls them with one shared service instance. Handwritten handlers, service/repository/model files and prior version packages are untouched. Reruns fail without overwriting. Resolve an edited generated registry before retrying.

`--policy replace` explicitly stops registering legacy routes, while retaining their source files. That choice persists when later versions are added. No command deletes v1. Deprecation and retirement are application decisions; announce retirement and remove old registrations only as an explicit breaking change.

## Choose a prefix

Default prefixes are /api/v1, /api/v2. Use `--prefix /custom/v1` when generating, or `http.WithVersion(registrar,"v1",prefix)` when registering manually. Versions are positive major identifiers (v1, v2, ...), never v0 or arbitrary path fragments. Version registration changes the typed contract and runtime path together, adds an operation-ID suffix and refuses collisions.

## Export independent contracts

/openapi.json describes legacy routes. /openapi/v1.json and /openapi/v2.json describe their respective effective paths and independent schemas; each document validates against OpenAPI 3.1. `App.OpenAPIVersion("v1")` exports a registered document without listening and rejects unknown versions. All public documents are disabled together with DisableOpenAPI. CLI export defaults to the legacy document; select a version with `gonex openapi generate --http-version v1 --out v1.json`.

## Start with a versioned module

Create an initially versioned module with `gonex create module catalog --http-version v1`. This profile starts directly with http/v1 and does not register legacy routes. Compare each released document using `gonex openapi validate --file current.json --against baseline.json`; see [compatibility policy](upgrade.md).

## Handle custom adapter layouts

Version generation expects the Phase 4 adapter Register signature and generated HTTP registry hook. Custom adapter layouts/signatures may require a manual registration change; conflicts must be resolved before generation. Copying a version does not infer a new business API: edit its schemas and handlers to define the intended compatibility boundary.

## RPC clients

Explicit module/client --version vN creates separate adapters; grpc generate --check checks drift. Multiple versions can share one caller-owned pool/config with generated WithConfig constructors. Unpublished placeholders are not release baselines. See [compatibility](microservices-dx/compatibility.md).
