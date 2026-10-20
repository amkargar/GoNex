# Gonex — Microservices DX & Simplicity Implementation Plan

**Status:** Proposed, not implemented  
**Repository:** `github.com/amkargar/gonex`  
**Focus:** Microservices developer experience (DX), KISS/DRY, optional gRPC and HTTP, explicit boundaries  
**Date:** 2026-10-20

## 1. Goal and non-goals

**Goal:** A developer can create two independently deployable services, define a versioned gRPC contract, implement a plain-Go use case, wire a client, and run both services locally with minimal manual configuration. The generated code must remain readable and editable, and deployment must not depend on local `replace` directives or `go.work`.

**Non-goals:** A custom RPC framework, service mesh, automatic domain mapping, implicit retries, production plaintext gRPC, distributed transactions, automatic service discovery, an obligatory database, obligatory HTTP/versioning, or a required central runtime.

**Core principle:** Simplify *workflow and generated boilerplate* without hiding network boundaries, errors, timeouts, ownership, or security. Do not introduce a global service locator or runtime reflection-based injection.

## 2. Existing implementation — preserve what works

Verified from repository documentation and implementation:

- `gonex new NAME --module ... --arch microservices` creates a workspace; `create service`, `create module`, `create client`, and `grpc generate` exist.
- Each service owns `go.mod`, configuration, `main.go`, module registry and deployments. Contracts are in a separate versioned module.
- Optional HTTP/gRPC modules and explicit static targets already exist. gRPC clients use a managed connection pool and typed generated clients.
- TLS is the default requirement; plaintext is explicitly development-only. Deadline, health, readiness, graceful shutdown, Buf/proto compatibility and isolated Go module checks already exist.
- Current `create client` leaves **business mapping and injection TODOs**. This is deliberate: the framework must never guess domain semantics.
- Workspace creation currently requires multiple commands plus manual `go mod tidy`; there are separate provider and consumer environment variables.

Do **not** replace these secure foundations in pursuit of shorter demos. This plan layers improved tooling and templates onto them.

## 3. Target first-run experience

Proposed **new** commands below are target UX and are **not current guarantees**.

```bash
gonex new commerce --module example.com/acme/commerce --arch microservices
cd commerce
go tool gonex create service identity --grpc
go tool gonex create service orders --http
go tool gonex create module users --service identity --transport grpc --version v1
go tool gonex create module orders --service orders --transport http
go tool gonex create client identity --service orders --module orders --version v1
go tool gonex dev
```

`dev` should concurrently start services with clear prefixed output, perform safe local prerequisite validation, and print HTTP/gRPC URLs. **It must not silently implement the consumer's mapping.** The generated TODO should be actionable and its diagnostics point to the exact file and docs. The complete tutorial should include the small mapping implementation, after which `gonex dev` can demonstrate `Orders → Identity`.

**One-process alternative:** Existing `gonex new shop ...` plus HTTP-only modules remains the simplest default. Never force microservices on users.

**No HTTP or no gRPC:** Each service can choose `--http`, `--grpc`, `--http --grpc`, or no transport initially (worker/library module); HTTP API versioning is only enabled when requested. CLI should reject contradictory flags with actionable errors.

## 4. Target workspace layout

```text
commerce/
├── go.mod                         # root CLI tool/module bookkeeping
├── go.work                        # OPTIONAL local convenience only
├── gonex.yaml                     # declarative service catalog, no secrets
├── contracts/
│   ├── go.mod
│   └── proto/identity/v1/users.proto
├── services/
│   ├── identity/
│   │   ├── go.mod
│   │   ├── cmd/server/main.go
│   │   ├── internal/bootstrap/wire.go
│   │   └── modules/users/
│   │       ├── module.go
│   │       ├── service.go
│   │       └── grpc/handler.go
│   └── orders/
│       ├── go.mod
│       ├── cmd/server/main.go
│       ├── internal/bootstrap/wire.go
│       └── modules/orders/
│           ├── module.go
│           ├── service.go
│           ├── http/schemas.go
│           ├── http/handlers.go
│           └── clients/identity.go # generated typed adapter skeleton
└── .gonex.json                    # generator ownership metadata
```

**Migration policy:** Keep existing generated layout working. Adopt changed paths only for a new opt-in template profile after golden tests; do not rewrite old projects automatically. `gonex.yaml` is a proposed addition. Never let it duplicate authoritative `go.mod` module versions or hand-written bootstrap business logic.

## 5. Simple developer API — proposed examples

### Domain boundary (plain Go)

```go
// services/orders/modules/orders/service.go
package orders

import "context"

type IdentityReader interface {
    GetUser(ctx context.Context, id string) (User, error)
}

type User struct { ID, Name string }

type Service struct { identity IdentityReader }

func NewService(identity IdentityReader) *Service {
    return &Service{identity: identity}
}
```

### Transport adapter (generated skeleton with one explicit mapping)

```go
// Example shape only — generated package names follow actual proto descriptors.
type IdentityAdapter struct {
    rpc identityv1.UserServiceClient
}

func (a *IdentityAdapter) GetUser(ctx context.Context, id string) (orders.User, error) {
    response, err := a.rpc.GetUser(ctx, &identityv1.GetUserRequest{Id: id})
    if err != nil { return orders.User{}, mapRPCError(err) }
    return orders.User{ID: response.Id, Name: response.Name}, nil
}
```

Only generate the exact request/response code when the contract is unambiguous and the mapping can be verified against known fields; otherwise emit an **explicit compile-time TODO and a clear scaffold diagnostic**, not invented mappings. Code should not pretend to compile while leaving a broken runtime wiring path. Prefer an explicitly named `example` template for illustrative mapping and a `skeleton` template for real custom business contracts.

### Connection management

Preserve existing `grpc.NewClients`, `grpc.NewClientConfig`, `grpc.Client[T]`, and `Clients.Ready`. Hide repetition in a *generated bootstrap helper*, not a second competing runtime RPC abstraction. One client config per target/security profile; reuse pool; no automatic retries on writes.

## 6. CLI contract

| Command | Purpose | Required behavior |
|---|---|---|
| `gonex new ... --arch microservices` | Workspace scaffold | Minimal sensible defaults, readable output |
| `gonex create service NAME --http/--grpc` | Independent deployable | Unique ports, isolated module, no obligatory transport |
| `gonex create module NAME --service S --transport ...` | Domain module + adapters | No proto imports in domain service |
| `gonex create client PROVIDER --service CONSUMER --module M --version v1` | Typed client adapter | Schema-accurate skeleton, explicit mapping ownership |
| `gonex grpc generate --check` | Regenerate/verify artifacts | Atomic staging, reproducible official stubs |
| `gonex dev [--service NAME]` **NEW** | Local multi-service runner | Child-process supervision, prefixed logs, graceful stop |
| `gonex doctor` **NEW** | Diagnose local configuration | Versions, ports, services, contracts, TLS flags; never print secrets |
| `gonex graph` **OPTIONAL** | Dependency edges | Static manifest analysis, no runtime discovery claims |

**Consistent flags:** `--service`, `--module`, `--version`, `--transport`, `--dry-run`, `--json` when applicable. `--help` must provide a runnable example and clear error codes. Avoid aliases until there is documented demand.

## 7. Proposed manifest (local developer orchestration only)

```yaml
schema: 1
services:
  identity:
    path: services/identity
    grpc: 127.0.0.1:9091
  orders:
    path: services/orders
    http: 127.0.0.1:8082
    clients:
      identity:
        address: 127.0.0.1:9091
        transport: grpc
```

Rules: forbid duplicate names/ports; reject unknown keys unless versioned explicitly; local loopback plaintext must require the existing explicit insecure flags (or an explicit `--dev-insecure` opt-in displayed prominently), never become production defaults. Environment variables override local addresses; credentials and tokens do not belong in manifest. Detect circular service dependencies and warn without automatically banning legitimate peer topology.

## 8. Implementation phases (execute in order)

### Phase 0 — Audit and baseline
**Inputs:** `docs/microservices.md`, `docs/grpc.md`, `internal/cli/`, `internal/scaffold/`, `grpc/`, `integration/rpc/`, `examples/microservices/`, tests.  
**Changes:** Inventory all current commands, template outputs, error messages and source-of-truth paths. Record end-to-end setup steps and friction, and capture current goldens.  
**Deliverables:** `docs/microservices-dx/baseline.md`, reproducible manual+automated baseline tests.  
**Acceptance:** Existing microservice generation, TLS/mTLS, readiness, grpc check and GOWORK=off verification remain green.

### Phase 1 — Ergonomic CLI & configuration
**Files:** `internal/cli/` (modify command parsing/help), `internal/scaffold/` (modify validation/templates), `internal/cli/microservices_test.go` (extend), **NEW** manifest parser under `internal/workspace/` (or equivalent).  
**Inputs:** workspace root, service names and transport selection.  
**Implementation:** Consistent flags and validations; auto-select unique ports; manifest round-trip; fail before writes on invalid input; `--dry-run` guaranteed side-effect-free; no hidden go.work requirement.  
**Tests:** golden outputs, conflicting ports, invalid names, duplicate generation, invalid manifest, reruns/idempotency, path traversal.  
**Outputs:** deterministic scaffolds and helpful errors.  
**Acceptance:** Two-service workspace can be generated with <10 clear CLI steps; no hand edits in framework-owned registries; all old flags and templates still supported.

### Phase 2 — Typed client scaffolding & explicit wiring
**Files:** `internal/scaffold/` client templates, `internal/cli/microservices_test.go`, `integration/rpc/`, new generator golden fixtures.  
**Inputs:** provider module and proto version, target consumer service, caller-owned interface.  
**Implementation:** Resolve descriptor exactly, detect ambiguous RPCs, generate adapter constructor and bootstrap injection point; never infer semantic field mapping. Provide documented choice of `skeleton` or explicit `example`. Manage client pool once in bootstrap.  
**Tests:** fully compiling example profile; intentional TODO skeleton has clear marker/error; two versions side-by-side; conflict-safe re-generation; provider unavailable maps to bounded failure.  
**Outputs:** understandable client adapter + single owner for wiring.  
**Acceptance:** User edits only business mapping and requested domain logic; no proto types leak into plain-Go service; TLS/timeouts stay configurable.

### Phase 3 — One-command local dev runner
**Files:** **NEW** `internal/devrunner/` (or equivalent), `internal/cli/` command, CLI integration tests, docs.  
**Inputs:** manifest and selected services.  
**Implementation:** Find workspace, validate dependencies/ports, optionally run prerequisite codegen/check *only on explicit flag*, run child processes with OS-safe cancellation, prefix stdout/stderr, handle failure and Ctrl+C, return non-zero on startup failure. No silent `go mod tidy` or network tool download on `dev`.  
**Tests:** startup, port conflict, worker failure, SIGINT/SIGTERM, repeated run, no orphan children, cross-platform Windows/Linux behavior.  
**Outputs:** `gonex dev` prints each service URL and a readable process summary.  
**Acceptance:** A fresh valid workspace starts with one command after dependencies are installed; stopping the runner stops all owned services.

### Phase 4 — Contract generation & API evolution UX
**Files:** existing contracts generator/compatibility, `internal/cli/`, docs, golden fixtures, versioning tests.  
**Inputs:** versioned proto definitions and baseline descriptor.  
**Implementation:** `grpc generate --check`, better compatibility diagnostics, atomic writes, clear separation of editable proto vs generated pb.go. Generated clients select explicit version; do not rewrite consumer mappings on upgrade.  
**Tests:** protobuf reproducibility, Buf breaking, generation twice same tree hash, incompatible field removal rejected, v1+v2 coexistence, clean module builds.  
**Outputs:** safe versioned codegen without mysterious edits.  
**Acceptance:** Breaking changes blocked and version upgrade path documented.

### Phase 5 — Production-safe defaults with optional simplicity
**Files:** `grpc/` (fix only evidenced defects), scaffold env templates, docs, test fixtures.  
**Inputs:** per-service TLS configuration, deadlines, health/readiness checks.  
**Implementation:** Dev insecure explicit; production TLS/mTLS required; default bounded timeout, clear unavailable error behavior, one client pool, graceful service shutdown; no automatic retries for mutating RPCs. Support HTTP-only, gRPC-only and mixed services.  
**Tests:** TLS/mTLS, cancellation, dependency down, stale/invalid CA, shutdown/drain under load, readiness, GOWORK=off.  
**Outputs:** low-boilerplate safe deployment templates.  
**Acceptance:** no production plaintext by default, no unbounded RPC calls, no hidden background child resources.

### Phase 6 — End-to-end onboarding & docs
**Files:** `examples/microservices/` (improve), **NEW** `docs/microservices-dx/quickstart.md`, `docs/microservices-dx/troubleshooting.md`, `docs/microservices-dx/compatibility.md`, `docs/microservices.md` (update), website docs (separate repo).  
**Inputs:** fresh installation and independent developer walkthrough.  
**Implementation:** show first success path before advanced config; use a real Orders→Identity mapped example; annotate generated vs user-owned files; 5-minute path without hiding security compromises.  
**Tests:** isolated clean checkout smoke, Linux/Windows, generated Go modules without replace, provider disconnect and recovery.  
**Outputs:** maintainable practical documentation and reproducible tutorial.  
**Acceptance:** an unfamiliar developer can follow Quickstart without repository knowledge; feedback and timing recorded, not assumed.

## 9. Testing matrix / hard acceptance gates

1. **Generator safety:** no overwrite of handwritten files, conflicts detected, dry-run zero writes, no path escape.
2. **Isolation:** each generated service builds with `GOWORK=off`; no production `replace` directives; shared contracts resolve by supported publication mechanism.
3. **Transport optionality:** HTTP-only, gRPC-only, both and no-transport profile each compiles and behaves as documented.
4. **Correctness:** provider/consumer startup, deterministic service selection, version-specific client, error mapping, contract validation.
5. **Security:** local insecure explicitly opt-in; release template requires TLS; mTLS verified; credentials never logged.
6. **Lifecycle:** interrupted startup, child exit, Ctrl+C, timeouts, client/server graceful shutdown; no leaked processes.
7. **Compatibility:** existing CLI users and template fixtures unchanged unless explicitly migrated; proto/Go API compatibility gates remain green.
8. **DX:** two services and a working mapped RPC can be reproduced from documentation with only user-owned business edits; CI validates exact tutorial commands.

CI jobs to add/extend: generated-workspace matrix (Linux/Windows), CLI fixture snapshots, child-process lifecycle runner, independent contracts publish/install smoke (or known-resolvable pinned commit), TLS/mTLS integration, Buf breaking, tutorial smoke.

## 10. Priorities and stop conditions

**P0:** truthful typed client scaffold; CLI errors; independent modules; secure defaults; deterministic generator; integration tests.  
**P1:** `gonex dev` and `doctor`; consolidated docs and examples.  
**P2:** dependency graph visualization, optional orchestration integrations. Avoid until P0/P1 are verified.

Do not add Kubernetes operators, a custom service mesh, magic auto-wiring, implicit retries, dynamic discovery, or required distributed infrastructure as part of the simplicity initiative.

## 11. Definition of Done

- An independently deployed Orders service calls Identity via a versioned gRPC contract while keeping domain code protocol-agnostic.
- One clear tutorial explains all user-owned edits; all generated files are deterministic and safely regenerable.
- Local developer experience is a single command **after** initial creation/mapping and dependency installation.
- CI validates Linux/Windows generated workspaces, isolated modules, failure scenarios and security boundaries.
- Existing monolith, HTTP-only and gRPC-free users experience no regression.
- An acceptance report names exact commits, commands, test results and remaining limitations; code existence alone is not treated as verified usability.

## 12. Documentation and website synchronization

Update framework docs (`docs/microservices.md`, `docs/grpc.md`, CLI reference, Getting Started, architecture and versioning) and matching pages in `GoNex-website`. Mark proposed features as proposed until merged and tested. Examples and commands across README, docs and website must match the actual CLI, not future design. Link the tutorial from the site homepage and development documentation once released.
