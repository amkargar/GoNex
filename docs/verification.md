# Reproducible verification

Use Go 1.27.2 or a later supported patched 1.27 release. From a clean checkout, run `bash scripts/verify.sh` on Linux/macOS, or `pwsh -File scripts/verify.ps1` on Windows. `GONEX_RACE=1` additionally runs the race suite on a supported platform with a C compiler. GOWORK is disabled deliberately. The source fixtures in integration/rpc and microservices contain explicit local source replacements; these are test-only, not evidence of an independently published install.

The quality workflow verifies Linux and Windows with the minimum patched Go version and latest 1.27.x; nested modules are checked independently. PostgreSQL 16 runs in an isolated CI service. `GONEX_REQUIRE_POSTGRES=1` turns a missing test DSN into failure; real-DB jobs repeat the integration, process-concurrency, cancellation and outage suites with the race detector. Other jobs exercise public CLI installation and generated apps using the PR's actual head commit, security scanning/fuzzing, Buf contracts and performance budgets.

For a public install rehearsal run `GONEX_SMOKE_VERSION=PUBLIC_COMMIT go run ./internal/tooling/consumer`. This uses a temporary module without replaces/workspaces, installs the CLI, generates/builds a project, and checks health/JSON/OpenAPI. The helper terminates its disposable server; graceful signal behavior is tested by runtime/lifecycle tests separately.

## Repository settings gate

Workflow YAML does not enforce merge protection. An administrator must configure a ruleset for main requiring a pull request and all selected quality/security/DB/contract/production checks. Disable bypass for ordinary contributors and record the exact required check contexts after their hosted runs. Demonstrate that an intentionally failing disposable PR cannot merge, then that a corrected PR is accepted. Do not merge the failing probe.

At initial assessment, the public ruleset listing was empty and the integration received HTTP 403 for classic main branch-protection reads. Configuration and failing/passing enforcement proof remain BLOCKED until owner evidence is supplied. Settings: https://github.com/amkargar/gonex/settings/rules

## Evidence

Every report must identify commit SHA, workflow/run URL, platform/toolchain, command, result and skips. Successful local Linux tests do not imply Windows/PostgreSQL success. Store test output as CI artifacts and pin links in docs/production-readiness. A generated workflow is not a green run. Reports must retain failures and material limitations.
