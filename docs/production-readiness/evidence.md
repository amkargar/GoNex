# Verification evidence

Baseline: e835251029c429b599c10da1b06e71dae37fab05; editor environment Linux amd64. The minimum toolchain was raised from Go 1.27.1 to 1.27.2 after a real govulncheck v1.9.0 run (database timestamp 2026-10-20T00:20:56Z). Reachable findings included standard-library HTTP/TLS issues, grpc transport issues, pgx placeholder parsing, QUIC and x/text. The patched root scan completed with no symbol-reachable findings according to internal/tooling/vulncheck; all-module scans are separately required.

Executed locally before hosted verification:

- Existing and new HTTP/security/metrics tests passed on Linux.
- SQLite CRUD/constraints/rollback and three-process migration/forward-recovery tests passed; PostgreSQL cases skipped locally because no local database is installed. Hosted required-DSN job is authoritative for PostgreSQL.
- Shutdown-during-startup, Nth-resource rollback and thirty lifecycle/concurrent-stop loops passed three times under the race detector after fixing listener ownership.
- Official apidiff comparison with e835251 reported no incompatible public API changes (source baseline, not a published release).
- Gitleaks v8.30.1 directory scan found no leaks; history scan is a separate hosted/security-script gate.
- First bounded POST workload: 2000 requests, 16 clients, zero errors. Short local figures are not production SLOs or long soak evidence; performance workflow retains reproducible environment/profiles.

Hosted links and exact final outcomes are added after runs complete. Pending/absent results are not passing evidence. No owner protection configuration, independent reviewer sign-off, prior stable release, signed tag, real deployment or independent developer onboarding is fabricated.
