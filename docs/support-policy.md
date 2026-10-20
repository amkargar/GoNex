# Support and compatibility policy

Gonex is pre-v1. Current verification targets are Linux amd64 and Windows amd64 on patched Go 1.27 (minimum 1.27.2), PostgreSQL 16, and the repository's file-backed SQLite driver. Latest 1.27.x is tested separately. Cross-compiled darwin/arm64 artifacts do not establish runtime certification. Record the exact hosted run and platform in production-readiness reports before claiming verification.

Before v1, minor versions can introduce reviewed breaking Go/CLI/template changes; patch updates should preserve contracts except documented security corrections. Independent HTTP and Protobuf majors retain their own compatibility rules. Deprecated behavior needs a replacement and a changelog/upgrade entry; there is no promised historical maintenance window before a release line is explicitly supported. No LTS or response-time SLA is implied.

Pin library and project-local CLI versions in go.mod/go.sum. Generators never silently rewrite handwritten code. Applied SQL migrations are immutable. Compare OpenAPI and Buf contracts against an actual published baseline once one exists; current v0.1.0-named fixture is not a published release. A release candidate is not a stable-production guarantee.

Release gates include quality/race/DB/security/contract checks, clean public install, candidate artifact provenance/checksums, production-like E2E/failure/upgrade evidence, configured merge protection and independent developer/security review. Unmet gates block the release decision even if an implementation PR is useful and mergeable. See production-readiness/final-report.md.
