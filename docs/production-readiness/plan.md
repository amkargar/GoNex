# Gonex — Production Readiness Implementation Plan

**Repository:** `github.com/amkargar/gonex`  
**Date:** 2026-10-20  
**Status:** Implementation specification / not evidence of completion  
**Goal:** Make Gonex a reliably releasable, API-first Go framework, with reproducible operational evidence.

## Principles

- Prioritize reliability over feature expansion. Do not introduce templating, admin UI, a custom ORM, or a message broker as part of this roadmap.
- Preserve API-first design, typed HTTP contracts, `schemas.go` DTOs, modular applications, and optional HTTP versioning, gRPC, and persistence.
- **Assess before implementing:** Existing facilities must be verified and hardened, not rewritten automatically.
- Tests and documentation are part of every feature change; a phase is complete only when its gates actually pass.
- Keep adapters opt-in and business logic testable without network infrastructure.
- Do not equate a configured CI workflow with evidence of a successful hosted run.

## Existing evidence / starting point

Reviewed repository files include `README.md`, `go.mod`, `app/app.go`, `http/router.go`, `http/binding.go`, `observability/logging.go`, `tests/operations_test.go`, `tests/security_test.go`, `tests/benchmark_test.go`, `.github/workflows/{test,contracts,release}.yml`, `docs/{operations,security,upgrade,release}.md`, `docs/phases/8.md`, and `docs/benchmarks/windows-amd64.txt`.

Existing functionality includes CLI scaffolding, Gin-backed HTTP, OpenAPI compatibility tooling, gRPC/Protobuf checks, GORM persistence, migrations, readiness, structured logs, error redaction and benchmark infrastructure. The phase-8 report documents local testing and explicitly leaves hosted CI validation, Linux race validation, live PostgreSQL validation, and public release verification outstanding. Reassess all claims against current CI results before closing gates.

## Dependency / execution order

```text
Phase 1 CI & quality gates
  ├─ Phase 2 Database reliability
  └─ Phase 3 Security hardening
       ↓
Phase 4 Observability & diagnostics
  ├─ Phase 5 Performance & stress
  └─ Phase 6 Resilience & lifecycle (also requires Phase 2)
       ↓
Phase 7 Compatibility & release engineering
       ↓
Phase 8 Production-like validation & release decision
```

Each phase delivers (1) implementation/changes, (2) automated tests, (3) reproducible verification evidence, (4) documentation, and (5) explicit pass/fail acceptance criteria. Files below are proposed additions or inspection targets, not guarantees they must be created if existing counterparts suffice.

---

# Phase 1 — CI, Build Reproducibility & Quality Gates

**Objective:** Prevent known regressions from entering `main`, and prove that clean installs and generated apps work.

**Inputs:** Entire repository; root/submodule `go.mod` files; workflows; existing tests, contract baselines, scaffolding templates.

### Files
| Path | Required action |
|---|---|
| `.github/workflows/test.yml` | Audit/fix matrix and jobs; verify actual hosted results |
| `.github/workflows/contracts.yml` | Audit Buf and generated-code diff checks |
| `.github/workflows/release.yml` | Audit only; activation belongs to Phase 7 |
| `.github/workflows/security.yml` | Add vulnerability/secret-scanning gate where absent |
| `scripts/verify.sh`, `scripts/verify.ps1` | Add repeatable local checks |
| `docs/verification.md` | Document commands, prerequisites, supported platforms, failures |
| `docs/release.md` | Record policy and support-matrix evidence |
| `internal/tooling/benchcheck/` | Confirm existing performance guard remains valid |

### Work
1. Inventory existing jobs and collect hosted evidence (commit SHA, run URL, OS, Go version, result).
2. Standardize format, `go mod verify`, build, vet, unit/integration tests and independent submodule checks; avoid local `go.work` masking dependencies.
3. Run race checks on a supported runner; establish an explicit OS/Go support matrix.
4. In a clean environment, install CLI, `gonex new`, create a module, resolve dependencies, build and run generated app; verify `/healthz` and `/openapi.json`.
5. Add auditable dependency checks, deterministic generated-code comparisons and artifact retention.
6. Configure required checks and branch/ruleset protection (a repository setting, not merely a YAML file).
7. Demonstrate a failing PR blocked and a passing PR accepted.

### Tests/commands
```sh
gofmt -l .
go mod verify
go test ./...
go test -race ./...     # supported platform
 go vet ./...
go build ./...
go run ./cmd/gonex grpc check
```
Also run independent `go -C <module> test ./...` for contracts, RPC integration and example services; run generated-project smoke tests.

**Outputs:** Green hosted CI matrix; blocked failing PR; clean-install smoke-test logs; verification scripts; versioned test artifacts.

**Acceptance criteria:**
- [ ] Every required CI job passes at a documented commit and platform.
- [ ] Required-status checks actually block a failing merge.
- [ ] All supported modules build/test independently without local replacements.
- [ ] Generated project runs from a clean environment.
- [ ] Race detector passes; no critical test is silently skipped.
- [ ] Failures are reproducible locally or on a documented CI runner.

**Exit gate:** Pass/fail PR demonstration and linked successful CI run.

---

# Phase 2 — Database Reliability & Migration Safety

**Objective:** Verify PostgreSQL/SQLite CRUD, transactions, migrations, concurrency, failures and cancellation using disposable databases.

**Inputs:** `orm/`, `migration/`, SQLite tests, PostgreSQL CI service, example repositories.

### Files
| Path | Required action |
|---|---|
| `orm/`, `orm/readiness.go` | Audit pool lifecycle, readiness, cancellation |
| `migration/runner.go` | Audit advisory locking, transactional behavior, failure states |
| `tests/database_integration_test.go` | Add/merge real-DB CRUD, constraints, transaction tests |
| `tests/database_failure_test.go` | Add connection loss, timeout and reconnection tests |
| `tests/migration_concurrency_test.go` | Add parallel-process migration tests |
| `tests/migration_recovery_test.go` | Add failed migration and immutable-history checks |
| `.github/workflows/test.yml` | Make PostgreSQL integration a non-optional release gate |
| `docs/database-reliability.md` | Publish guarantees, unsupported operations and recovery advice |

### Work
1. Run real PostgreSQL integration tests against an ephemeral service and assert they did not skip.
2. Test CRUD, unique/foreign key constraints, transactions, rollback and cancellation.
3. Run migrations concurrently from separate processes and verify no duplicate application.
4. Inject failures mid-migration; document DB-specific DDL transactional limits.
5. Confirm migration history/change checks, readiness while DB unavailable and recovery after restoration.
6. Distinguish framework migration semantics from app-specific backup/rollback responsibilities.

### Scenarios / expected result
| Scenario | Expected |
|---|---|
| Successful transaction | Commit |
| Transaction failure | Rollback |
| Cancellation | Context-aware, bounded result |
| Concurrent migration | Single valid application |
| Migration failure | Never marked successful |
| PostgreSQL unavailable | Bounded, visible failure/readiness behavior |
| PostgreSQL restored | Recovery per documented pool policy |
| Previously applied migration modified | Incompatibility surfaced, not silently accepted |

**Outputs:** Deterministic PostgreSQL+SQLite tests, migration concurrency report, database operations guide.

**Acceptance criteria:**
- [ ] Real PostgreSQL CI test passes without skip.
- [ ] SQLite suite passes with no regression.
- [ ] Concurrent migration is safe on supported DB engines.
- [ ] Error paths roll back or report partial effects correctly.
- [ ] Cancellation/readiness behavior is bounded and documented.
- [ ] Backups/forward recovery responsibilities are explicit.

**Exit gate:** Repeated real-DB failure/integration suite runs successfully.

---

# Phase 3 — Security Hardening

**Objective:** Validate untrusted input boundaries, authentication/authorization adapters, transport security and secret handling.

**Inputs:** `security/`, `http/binding.go`, `http/json.go`, `grpc/`, logging, CLI/scaffolding and existing security tests.

### Files
| Path | Required action |
|---|---|
| `http/binding.go`, `http/json.go` | Fuzz/audit strict parsing and resource limits |
| `security/` | Validate authn/authz, CORS and policy ordering |
| `grpc/` | Check TLS/mTLS, insecure opt-in, recovery and deadlines |
| `observability/logging.go` | Verify secret redaction and bounded fields |
| `tests/security_test.go` | Expand with adversarial cases |
| `http/binding_fuzz_test.go` | Add parser fuzz targets and regression corpus |
| `tests/security_boundary_test.go` | Add cross-boundary negative cases |
| `.github/workflows/security.yml` | Add scan/fuzz smoke gates |
| `SECURITY.md` | Add private disclosure and supported-version policy |
| `docs/security.md` | Document shared responsibility and safe defaults |

### Work
1. Produce small threat model for HTTP, gRPC metadata, CLI paths, config and DB errors.
2. Fuzz malformed/deep JSON, duplicate keys, Unicode, unexpected types and query/path bindings with bounded resource expectations.
3. Test 401/403 distinction, middleware ordering, empty principals, deny-by-default policies and CORS preflight.
4. Test TLS minimums, certificate verification, mTLS and explicit dev-only plaintext.
5. Test panic/internal errors/logs for credential, header, body and stack leakage.
6. Run `govulncheck` across all Go modules, triage reachable findings; document unresolved risks.
7. Publish `SECURITY.md` and vulnerability-response process.

### Tests
```sh
go test ./...
go test -race ./...
go test ./http -run '^$' -fuzz '^Fuzz' -fuzztime=60s
# execute separately inside each applicable module:
govulncheck ./...
```

**Outputs:** Threat model, fuzz corpus, security regression tests, scan report, disclosure policy.

**Acceptance criteria:**
- [ ] No unmitigated release-blocking Critical/High reachable finding.
- [ ] Bounded fuzz campaign passes without crash or newly discovered bug.
- [ ] Input/body limits resist bypass and avoid disproportionate resource use.
- [ ] Internal errors and logs do not leak known test secrets.
- [ ] Unsafe gRPC transport remains explicit opt-in.
- [ ] Authn/authz and CORS positive/negative scenarios pass.
- [ ] Security disclosure route published.

**Exit gate:** Security review signed off with known residual risks recorded.

---

# Phase 4 — Observability & Diagnostics

**Objective:** Offer coherent logging, metrics and traces without imposing external infrastructure on simple APIs.

**Inputs:** `observability/`, `observability/grpc/`, `app/readiness.go`, current operations/logging docs.

### Files
| Path | Required action |
|---|---|
| `observability/logging.go` | Review structured field schema and redaction |
| `observability/` metrics implementation | Extend bounded request/error/inflight/duration measurements |
| `observability/grpc/` | Test unary propagation; decide/document stream support |
| `observability/otel/` | Optional OTEL adapter if justified (avoid hard core dependency) |
| `tests/observability_integration_test.go` | Validate HTTP→service→gRPC trace continuity |
| `examples/observability/` | Runnable collector/exporter example |
| `docs/observability.md`, `docs/operations.md` | Configuration, privacy, dashboards, troubleshooting |

### Work
1. Define stable JSON fields for service, request/trace IDs, method, route template, status, duration, error code.
2. Preserve human-readable dev logging; default production logging to structured JSON with configurable levels.
3. Keep raw URLs, bodies, credentials and user-identifying data out of default labels/log fields.
4. Add opt-in, standards-compatible exporter adapter, preserving no-exporter startup.
5. Provide appropriate latency histograms and cardinality budgets; verify concurrency safety.
6. Trace HTTP→gRPC propagation, and implement streaming instrumentation only if declared supported.
7. Demonstrate troubleshooting a forced dependency failure from logs, metrics and traces.

**Outputs:** Observability contract, optional adapter, runnable example, bounded metrics, diagnostics guide, measured overhead.

**Acceptance criteria:**
- [ ] One expected completion log per request and usable correlation IDs.
- [ ] Trace propagation works for declared supported transports.
- [ ] No sensitive data in default logs/metric labels.
- [ ] Failed exporter cannot crash app or indefinitely block requests.
- [ ] Metrics are bounded and concurrency safe.
- [ ] Telemetry can be disabled; simple app requires no external backend.
- [ ] Diagnostic walk-through succeeds in a runnable example.

**Exit gate:** One injected request failure traceable end to end by operators.

---

# Phase 5 — Performance & Stress Testing

**Objective:** Record fair, reproducible performance cost and detect regressions before release.

**Inputs:** `tests/benchmark_test.go`, `internal/tooling/benchcheck/`, `docs/benchmarks/windows-amd64.txt`, HTTP/DB/RPC workloads.

### Files
| Path | Required action |
|---|---|
| `tests/benchmark_test.go` | Validate paired Gin/Gonex and GORM/Gonex parity |
| `internal/tooling/benchcheck/` | Make robust regression budgets and stats |
| `tests/load/`, `benchmarks/scenarios/` | Add end-to-end load harness |
| `benchmarks/results/`, `docs/benchmarks/` | Version baselines/hardware/environment/results |
| `.github/workflows/performance.yml` | Add controlled benchmark gating/reporting |

### Work
1. Pin reference commit, hardware, Go version, Gin mode, middleware and dataset.
2. Compare equivalent handler/payloads; measure core overhead separately from optional logging/telemetry.
3. Test simple GET, validated JSON POST, typed error response, routed parameters, pooled DB calls and gRPC.
4. Collect throughput, p50/p95/p99, CPU, heap, bytes/op, allocs/op, error rate.
5. Run burst, saturation, concurrency and bounded soak tests; identify leaks and tail-latency cliffs.
6. Introduce initial review thresholds (e.g. >10% sustained regression triggers investigation), then tune to statistically stable baseline.

### Example commands
```sh
go test ./tests -run '^$' -bench BenchmarkComparison -benchmem -count=5
# load test harness commands must be documented alongside their scenarios
```

**Outputs:** Reproducible measurements, reference workload definitions, regression budgets and bottleneck report.

**Acceptance criteria:**
- [ ] Benchmark environment and baseline commit recorded.
- [ ] Compared workloads are semantically equivalent.
- [ ] Tail latency and error rate are measured under representative load.
- [ ] Core and observability overhead reported separately.
- [ ] No unexplained sustained memory growth.
- [ ] Stable regressions over budget block/review release.

**Exit gate:** Independent rerun reproduces findings within stated variation.

---

# Phase 6 — Resilience, Fault Injection & Lifecycle

**Objective:** Establish bounded, documented failure, shutdown and cleanup behavior.

**Inputs:** `app/app.go`, `app/readiness.go`, `orm/readiness.go`, `grpc/`, `tests/operations_test.go`, example microservices.

### Files
| Path | Required action |
|---|---|
| `app/app.go`, `app/readiness.go` | Audit state transitions, startup rollback and concurrency |
| `grpc/` | Verify graceful/forced stop, deadlines and connection errors |
| `tests/lifecycle_test.go` | Exercise allowed/disallowed transitions |
| `tests/failure_injection_test.go` | Inject failed resource starts/stops and network outages |
| `tests/shutdown_stress_test.go` | Active requests during termination |
| `tests/resource_cleanup_test.go` | Detect leaked handles/goroutines |
| `docs/resilience.md` | Explicit lifecycle contract and operational runbook |

### Work
1. Test Created→Registered→Starting→Running→Stopping→Stopped and illegal transition errors.
2. Simulate failure on Nth resource startup; verify reverse cleanup and failure transparency.
3. Stop HTTP while in-flight calls execute; refuse new work and respect shutdown budget.
4. Test concurrent shutdown/readiness and idempotence policy under race detector.
5. Propagate cancellation through compliant handlers, DB and gRPC calls.
6. Fail/recover downstream PostgreSQL and RPC, observe readiness and bounded responses.
7. Run repeated lifecycle loops and check goroutine/socket/file-handle stability.

**Outputs:** Failure-injection suite, tested lifecycle contract, cleanup report, outage runbook.

**Acceptance criteria:**
- [ ] Illegal state transitions fail predictably.
- [ ] Startup failures clean previously started resources.
- [ ] Graceful shutdown drains until deadline, then takes documented fallback.
- [ ] Concurrent operations have no known data races or deadlocks.
- [ ] Cancellation propagates within framework-controlled boundaries.
- [ ] Readiness accurately reflects tested dependencies.
- [ ] Repeated lifecycle test shows no unexplained leak.

**Exit gate:** Automated fault scenarios pass repeatedly on supported runners.

---

# Phase 7 — Compatibility, Versioning & Release Engineering

**Objective:** Make installs, upgrades, contracts and release artifacts deterministic and reviewable.

**Inputs:** `openapi/{compatibility,freeze}.go`, `contracts/`, `internal/scaffold/`, `.github/workflows/release.yml`, `CHANGELOG.md`, `docs/{release,upgrade}.md`.

### Files
| Path | Required action |
|---|---|
| `openapi/compatibility.go`, `openapi/freeze.go` | Audit existing contract comparison |
| `contracts/baseline.binpb`, `contracts/proto/` | Pin published baseline and Buf compatibility |
| `internal/scaffold/` | Audit non-destructive generation and version profiles |
| `tests/compatibility_test.go` | Expand OpenAPI/Go/gRPC compatibility assertions |
| `tests/cli_upgrade_test.go` | Add real upgrade rehearsal |
| `tests/scaffold_compatibility_test.go` | Ensure human edits survive generation |
| `.github/workflows/release.yml` | Finalize gated builds, signing/checksums as appropriate |
| `CHANGELOG.md`, `docs/release.md`, `docs/upgrade.md` | Release notes and upgrade contract |
| `docs/support-policy.md` | Supported Go/OS/DB matrix, deprecation and patch policy |

### Work
1. Inventory exported Go API, CLI flags/subcommands, template output, HTTP and Protobuf contracts.
2. Compare Go APIs against previous released tag and classify intended breaking changes.
3. Compare OpenAPI method/path, params, schemas, responses and security with baseline.
4. Enforce Buf breaking and reproducible Protobuf generation against *published* baseline.
5. Validate generator conflict handling; never silently overwrite handwritten files.
6. Build CLI binaries with embedded version, SHA256 manifests and immutable provenance.
7. Test clean `go install ...@<tag>`, scaffold, build and version upgrade without local replaces.
8. Publish clear pre-v1 SemVer and support/deprecation policy; tag independent contracts module correctly when necessary.

### Tests
```sh
go test ./...
go vet ./...
go build ./...
go run ./cmd/gonex grpc check
go run ./cmd/gonex openapi validate \
  --file tests/testdata/basic.openapi.json \
  --against tests/testdata/releases/v0.1.0.openapi.json
# plus Buf lint/breaking, public API diff, generation snapshots and clean-tag install
```

**Outputs:** Reproducible candidate artifacts, versioned baselines, changelog, compatibility evidence, upgrade guide and support policy.

**Acceptance criteria:**
- [ ] All artifacts map to a reviewed commit/tag.
- [ ] Clean tagged install succeeds without local workspaces/replaces.
- [ ] Accidental breaking changes are detected before tagging.
- [ ] HTTP and Protobuf checks compare against proper release baselines.
- [ ] Scaffold does not overwrite handwritten files silently.
- [ ] Checksums, upgrade notes and support matrix are published.
- [ ] Release pipeline actually ran and passed, not just parsed successfully.

**Exit gate:** Install+upgrade test passes for a released/candidate tag in isolated environment.

---

# Phase 8 — Production-like Validation & Release Decision

**Objective:** Exercise the framework in an independently installable, realistic service and make a documented release decision.

**Inputs:** All previous gate evidence; a release candidate; disposable PostgreSQL, optional gRPC service, load and fault harness.

### Files
| Path | Required action |
|---|---|
| `examples/production/` (or separate sample repo) | Create realistic app, standalone `go.mod` |
| `examples/production/cmd/api/main.go` | App startup and shutdown |
| `examples/production/modules/identity/` | Authn/authz, schemas, handlers, service, repository |
| `examples/production/modules/orders/` | API and persistence workflow |
| `examples/production/migrations/` | Versioned schema |
| `examples/production/deployments/{Dockerfile,compose.yaml}` | Reproducible deployment |
| `examples/production/tests/{integration,e2e}_test.go` | DB and HTTP end-to-end validation |
| `docs/{deployment,production-checklist,troubleshooting}.md` | Production runbooks |
| `docs/release.md`, `docs/upgrade.md`, `CHANGELOG.md`, `README.md` | Final review |
| `docs/production-readiness/final-report.md` | Evidence-based final gate report |

### Work
1. Implement CRUD, pagination/filtering where supported, authentication, validation, database migration and OpenAPI on the pinned release candidate.
2. Demonstrate business-service tests without HTTP server.
3. Deploy the example with real PostgreSQL, secrets/config and health/readiness checks in a production-like environment.
4. Exercise E2E workflows and HTTP→database/gRPC paths with failure injection.
5. Load test under documented hardware and traffic expectations.
6. Practice a framework-version upgrade and application binary rollback; do not assume automatic DB-schema rollback.
7. Ask at least one developer unfamiliar with the project to follow public docs and record friction.
8. Review open security/compatibility/reliability risks; release only when blockers are closed.

**Outputs:** Pinned production-like sample, E2E suite, failure/load/upgrade/rollback report, published install documentation, formal readiness decision.

**Acceptance criteria:**
- [ ] Phases 1–7 have linked passing evidence.
- [ ] Independent pinned-tag install/build/run works.
- [ ] Real PostgreSQL-backed E2E tests pass.
- [ ] Injected outages match documented behavior.
- [ ] Upgrade and recovery drills succeed with documented limits.
- [ ] No unresolved release-blocking issues.
- [ ] Independent developer completes public quick start.
- [ ] Supported deployment scope is explicit; unsupported contexts are not claimed.

**Exit gate:** Signed-off release-readiness report and reproducible candidate release.

---

## Recommended tracking files

```text
docs/production-readiness/
├── README.md
├── phase-01-ci.md
├── phase-02-database.md
├── phase-03-security.md
├── phase-04-observability.md
├── phase-05-performance.md
├── phase-06-resilience.md
├── phase-07-release.md
├── phase-08-validation.md
└── final-report.md
```

Each phase report must include: **baseline status**, **exact changed files**, **tests and command output**, **run/commit links**, **known limitations**, **acceptance verdict**, **remaining actions**. Suggested statuses: `NOT_STARTED`, `IN_PROGRESS`, `BLOCKED`, `VERIFIED`.

## Release-blocker policy

Release must be blocked by: critical reachable vulnerability; failed required CI; failed real supported-DB integration; contract break outside versioning policy; reproducible data loss or unsafe migration; unexpected secret exposure; unbounded framework-controlled shutdown failure; unreproducible tagged install; major unexplained sustained performance regression outside published budget.

## Final evidence checklist

- [ ] Hosted CI and protected-branch evidence
- [ ] Real database integration and migration safety
- [ ] Security review, scans, fuzz regressions, disclosure policy
- [ ] Structured observability and diagnostic example
- [ ] Fair performance baselines and stress results
- [ ] Lifecycle, failure injection and leak/race validation
- [ ] Tagged release, checksums, compatibility/upgrade evidence
- [ ] Independent production-like sample and recovery drills
- [ ] Published support scope, limitations and release notes

**Interpretation:** Passing this plan establishes production-readiness for a *defined* support scope. A claim of 10/10 sustained operational maturity additionally needs evidence from real users, multiple upgrade/release cycles, ongoing maintenance, and incident resolution. Do not mark historical maturity complete solely because initial technical gates pass.
