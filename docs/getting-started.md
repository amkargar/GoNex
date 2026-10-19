# Getting started

Install the CLI from a published version with `go install github.com/amkargar/gonex/cmd/gonex@VERSION`, or build the checkout with `go build -o gonex.exe ./cmd/gonex` on Windows (`-o gonex` elsewhere). This checkout has not yet been published; generated projects default to v0.1.0 and accept `--framework-version` for the release you use. Development dependencies require an explicit local override; templates never embed this machine's paths.

```sh
gonex new shop --module example.com/acme/shop
cd shop
go mod tidy
gonex create module users
gonex create module reports --transport none
gonex check
gonex run
```

GET /users returns `{"message":"users"}`. GET /healthz returns `{"status":"ok"}`. OpenAPI is available at /openapi.json. Stop with Ctrl+C. `run` invokes Go directly; `build` builds all packages; `check` tests, vets and builds, preserving child failure exit codes. Go module downloads require network access or a populated module cache.

Use `--dry-run` on new/create commands to inspect plans without creating files. Handwritten source is never overwritten. `.gonex.json` records template format and hashes for tool-owned registries; edits to those files cause a conflict before any write. Existing projects/directories are refused. Writes are staged, guarded with an exclusive generator lock and rolled back on application failures. Interrupted OS/process failures may require inspecting `.gonex.lock`; multi-file filesystem writes are not crash-atomic. Symlinks and path traversal are rejected. Project names are lowercase Go identifiers; module paths and framework versions follow Go's own validation.

`gonex create module messages --db` adds a local Store interface, GORM adapter/model and explicit migration with a seeded message. Export DATABASE_URL and optionally DATABASE_DIALECT, run `gonex db migrate`, then serve. `gonex db status` returns migration history as JSON. No automatic .env loading. See [database guide](database-migrations.md).

`gonex openapi generate --out openapi.json` executes the project's initialization/export path without listening, validates the result and writes the JSON artifact. `gonex openapi validate --file openapi.json` validates offline. Both export and migration support dry runs without executing the project or connecting to the database.

Core-only modules have only module/service/test files. HTTP adapters contain handlers.go, schemas.go, routes.go and openapi.go. The extra http_registry_gen.go owns HTTP registration so future adapter versions can share the same service safely. Add your business behavior in the handwritten files.
