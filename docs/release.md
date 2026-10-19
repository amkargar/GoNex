# Release and support policy

This checkout is not a published release. Local Windows/amd64 verification is recorded in the phase reports. Linux/Windows Go 1.27.1 and latest 1.27 patch are configured CI targets; successful hosted runs are required before claiming that support matrix is verified. Other Go releases and operating systems are unverified. Release builds target Linux, Windows and macOS on amd64/arm64; cross-building alone does not prove runtime support.

Before a release:

1. Run the test workflow on the release commit: formatting, module verification, tests/vet/build, generated projects, real SQLite, disposable PostgreSQL, Linux race detector, OpenAPI validation/compatibility and benchmark budgets.
2. Run the contracts workflow: pinned official Buf lint/breaking, internal compatibility checks and reproducible official Protobuf generation with a clean generated diff.
3. Review changelog, upgrade instructions, security defaults, examples and all public contract baselines. Baseline updates require review against the previous release, rather than bypassing compatibility checks.
4. Resolve the public repository/module location, choose a semantic tag, and verify generated projects against the resolvable version without local replacements. Publish the independent contracts module under its own module tag when changed.
5. Tag only the reviewed commit. The release workflow reruns reusable test/contracts gates, cross-builds the CLI with an embedded version and uploads checksummed artifacts. It does not publish a GitHub release or push tags.
6. Inspect artifacts and hosted results before separately authorizing publication. Record the actual support matrix and release notes.

Only the latest published patch in the active release line is intended to receive fixes; there are no published supported releases yet. Security reports should use a private reporting channel established by the repository owner before publication; do not place credentials or exploit details in public issues. See [security configuration](security.md).

Known limits: static service targets, cooperative cancellation, additive migrations without automatic rollback, conservative compatibility checks, no automatic business mapping, no distributed tracing exporter or streaming trace propagation. Brokers, an admin panel and advanced discovery remain future work.
