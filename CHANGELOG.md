# Changelog

## Unreleased

- Raise the minimum Go patch to 1.27.2 and update reachable-vulnerability fixes in gRPC, pgx, QUIC and x/* dependencies.
- Add reproducible security/fuzz, public-consumer, real-DB failure/concurrency and performance gates.
- Harden ambiguous CORS/identity inputs and UTF-8 decoding; detach principal role slices.
- Add bounded latency histograms, opt-in Prometheus scraping and service/trace correlation.
- Fix concurrent startup/shutdown cancellation and explicit listener cleanup.
- Add a standalone pinned Orders API, disposable PostgreSQL/Compose recovery rehearsal and readiness reports.


- API-first library/CLI onboarding without cloning the framework.
- Installed-module version defaults and project-local Go tool dependencies for generated apps/workspaces.
- Structured slog lifecycle/request logging, bound startup URLs, request IDs and production JSON.

- Explicit module registration, managed lifecycle, readiness and Gin routing.
- Typed HTTP binding/validation, strict JSON, OpenAPI 3.1 and independent HTTP versions.
- Optional GORM persistence, PostgreSQL/SQLite and explicit checked migrations.
- Safe CLI project/module/version/database generators and deterministic templates.
- Optional TLS/mTLS gRPC, managed typed clients, versioned contracts and official stub generation.
- Independent microservice modules, descriptor-derived client skeletons and runnable examples.
- Trace propagation, request/error/latency metrics, auth/authorization and explicit CORS.
- Compatibility baselines, measured benchmark budgets and gated CI/release workflows.

No release has been published. See [upgrade notes](docs/upgrade.md) for strict DTO validation and template compatibility.

### Microservices developer experience

- Added strict local catalog, explicit listener profiles, doctor and supervised dev.
- Added descriptor-derived client profiles, bootstrap helpers, version-specific adapters and generation drift checks.
- Preserved existing paths/TLS/deadlines; added public tutorial and process lifecycle verification.
