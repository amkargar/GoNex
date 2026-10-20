# Verification evidence

## Source and scope

Baseline: `e835251029c429b599c10da1b06e71dae37fab05`. Implementation: `a6bc2c8`, `5197735`, `a355430`, `8ffe00e0ce7a2cb63abb652643cadf8517abee41`. The last includes the complete standalone sample and public upgrade rehearsal. The subsequent `5e54908998e7b8bd1e4c1dfc9915f33ab28da91e` changes only workflow lint and operations documentation. Reports are documentation-only additions after those commits.

Supported test scope is Linux/Windows amd64, Go 1.27.2 and latest 1.27.x, PostgreSQL 16 and file-backed SQLite. Cross-compilation is not runtime certification. Local editor results are Linux amd64; PostgreSQL and Docker results below come from actual hosted runners.

## Retained hosted results at 8ffe00e0ce7a

| Verification | Actual result and evidence |
| --- | --- |
| PostgreSQL reliability | [Required PostgreSQL race job](https://github.com/amkargar/GoNex/actions/runs/39392819018/job/118528204842) passed: CRUD/constraints/cancellation, independent migration processes, failed-DDL recovery and interrupted pool/recovery. Required DSN prevents silent skip. |
| Windows minimum/latest | [Minimum](https://github.com/amkargar/GoNex/actions/runs/39392819018/job/118528204838) and [latest](https://github.com/amkargar/GoNex/actions/runs/39392819018/job/118528204923) passed; minimum also executes verify.ps1. |
| Linux latest and race | [Linux latest](https://github.com/amkargar/GoNex/actions/runs/39392819018/job/118528205116) passed, including quality/race and independent modules. |
| Public consumer and real upgrade | [Consumer](https://github.com/amkargar/GoNex/actions/runs/39392819018/job/118528204451) passed: public installation outside the checkout, generated application, old source snapshot to current upgrade, handwritten code retained, project-local CLI, new module, build/health/API/OpenAPI. No workspace or replace. |
| Security | [Security run](https://github.com/amkargar/GoNex/actions/runs/39392818969) passed all-module reachable-vulnerability gates, history secret scan and bounded 60-second JSON fuzz; scan artifacts retained by workflow. |
| Contracts | [Contracts run](https://github.com/amkargar/GoNex/actions/runs/39392818911) passed official Go apidiff, HTTP/OpenAPI contracts and Buf compatibility. Source baseline is not mislabeled as a published release. |
| Performance | [Performance run](https://github.com/amkargar/GoNex/actions/runs/39392818979) passed paired benchmark ceilings and bounded load scenarios; uploaded environment, samples, tail latency, error counts, CPU/heap profiles and memory observations. Short hosted soak is not deployment certification. |
| Standalone PostgreSQL API | [PostgreSQL E2E](https://github.com/amkargar/GoNex/actions/runs/39392819011/job/118528204113) passed twice under race with required real PostgreSQL: authn/authz, CRUD, Unicode validation, filtering/pagination, contract and database-unready behavior. |
| Actual container rehearsal | [Compose recovery](https://github.com/amkargar/GoNex/actions/runs/39392819011/job/118528204347) passed actual image build, explicit migration, HTTP E2E, PostgreSQL stop, readiness 503, restore to 200 and retained-data verification. |

The aggregate Go and release-artifact runs at this commit failed **workflow lint**, despite the above passing jobs. The lint error was an unused shell loop variable, fixed at 5e54908. Those aggregate runs must not be reported green. Candidate cross-build success is recorded separately after its dependent gates finish.

## Local corroboration

- `scripts/verify.sh` and root/nested-module tests, vet, build and race passed on patched Go 1.27.2. SQLite CRUD/constraints/rollback and three-process migration/forward-recovery tests passed; local PostgreSQL was unavailable, so hosted results above are authoritative.
- Shutdown during startup, Nth-resource rollback and thirty lifecycle/concurrent-stop loops passed three race repetitions; listener ownership and nonblocking asynchronous failure notification were fixed.
- Runnable observability example was exercised over real HTTP: injected dependency failure yielded a correlated trace/log, error metrics and readiness 503; restoration yielded readiness 200 and graceful stop. HTTP-to-unary-gRPC propagation and telemetry/cardinality/exporter regressions passed.
- The public `8ffe00e0ce7a` install/upgrade rehearsal independently passed locally, including a live generated API. This is an upgrade between public source snapshots, not a claimed stable-release upgrade.
- A 60-second local fuzz campaign passed 59,305 executions without crash. Short local gRPC/SQLite load scenarios each completed 2,000 requests without errors; a paced 12-second probe completed 1,197 requests without errors. Hosted artifacts carry the reproducible performance measurements; these probes establish no production SLO.
- An initial real govulncheck v1.9.0 run (database 2026-10-20T00:20:56Z) found reachable standard-library/HTTP/TLS, gRPC, pgx, QUIC and x/text vulnerabilities. All modules and scaffold minimums moved from Go 1.27.1 to 1.27.2 and patched dependencies. Repeated all-module scans then passed the fail-closed symbol-reachable gate. Gitleaks v8.30.1 found no secrets in tracked content/history.

## Gates outside automated evidence

No owner protection configuration, independent security sign-off, private-channel activation, unfamiliar developer onboarding, prior stable release, signed tag verification or controlled long-duration deployment is fabricated. See [final decision](final-report.md). All CI artifacts have finite retention; archive relevant evidence before promotion.
