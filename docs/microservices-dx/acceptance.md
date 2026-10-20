# Microservices DX implementation acceptance

Specification: attached Microservices DX & Simplicity plan, 2026-10-20. Baseline: fb44296864d8798b86651854465cf10f3b7a677e. [Implementation PR and exact final hosted evidence](https://github.com/amkargar/GoNex/pull/2). Workflow configuration alone is not success.

| Phase | Implementation and verification scope |
| --- | --- |
| 0 Audit | baseline.md, retained original goldens and generated E2E. |
| 1 Catalog/CLI | Strict bounded YAML, listener profiles, deterministic port selection, flag conflicts and dry-run safety. |
| 2 Clients | Official descriptor-derived identifiers, skeleton/example/raw profiles, version outputs, explicit helper injection and mapping/wiring diagnostics. |
| 3 Runner | Cached read-only builds, explicit generation, prefixed bounded logs, Unix process groups/Windows Job Objects, failure/cancellation/descendant/repeat tests. |
| 4 Evolution | Generation drift check, compatibility baselines, coexisting v1/v2 and preservation of existing mapping. |
| 5 Defaults | Existing TLS/mTLS/deadline tests, no secret diagnostics, explicit local insecurity, all four listener profiles. |
| 6 Onboarding | Exact mapped tutorial, local-workspace preparation, immutable contracts publication fixture, isolated modules and website synchronization. |

Focused Linux race tests and original generated-workspace baseline passed. A later public-install rehearsal caught a generated cleanup call naming the wrong pool method; it was corrected to Close and the full path is repeated on the corrected head. Initial failed runs remain visible.

Public smoke installs a real public CLI commit, verifies local first-run builds, publishes a test-owned contracts zip/mod/info to a disposable file proxy with SHA256, resolves GOWORK=off/no-replace services, compiles coexisting contract versions/all transports, checks doctor/codegen, executes Orders→Identity and stops owned listeners. Source-only tests with explicit replacements are separate evidence. The classic E2E also kills/restarts the provider and checks bounded failure/recovery.

Independent unfamiliar-developer onboarding and elapsed time remain UNVERIFIED. No five-minute result, usability score, public contracts release, signed release or production-history claim is invented. Static wiring diagnostics do not prove business correctness or a live TLS handshake. Optional graph/path-template work is deferred. This implementation does not close the separate production-release gates in ../production-readiness/final-report.md.
