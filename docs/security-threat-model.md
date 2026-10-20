# Security boundary review

| Boundary | Threat | Controls and evidence |
| --- | --- | --- |
| HTTP JSON | Duplicate/unknown fields, invalid UTF-8, deep input, oversized bodies | Strict body decoding, 64-level nesting guard, endpoint byte budget; adversarial tests and FuzzStrictBinding |
| Auth | Empty identity, role aliasing, missing/wrong policy order | Reject whitespace-only subjects, detached role slices, 401 before identity and 403 for policy rejection; security regression tests |
| CORS | Conflicting origins/preflight headers, wildcard credentials | Explicit origins/method/header lists and ambiguous-header rejection; boundary tests |
| RPC | Forged peer, plaintext exposure, unbounded work | TLS minimum and certificate verification, explicit development-insecure opt-in, bounded deadlines and recovery; TLS integration suite |
| Telemetry | Credential/URL leakage, label-cardinality exhaustion | Route templates, no bodies/headers/query values by default, bounded operation labels and histogram buckets; log/exporter tests |
| CLI | Traversal, symlinks, overwrites, malicious generated source | Validated module/names, staged plans, ownership hashes, generation tests |
| Database | DSN disclosure, invalid history, concurrent migration | Redacted connection/readiness errors, immutable history and cross-process locking tests |
| Supply chain | Vulnerable dependencies, leaked credentials, tampered artifact | Pinned tools, reachable vulnerability gate, redacted secret scans, checksums and tagged provenance attestation |

The application chooses authentication verifiers, authorization predicates, identity providers and transport exposure. Gonex does not implement password storage or issue JWTs automatically. CORS is not authorization or CSRF protection. Default log fields are bounded but application-supplied logger attributes/messages can contain secrets; do not log credentials or personal data. Resource callbacks and custom writers must honor their documented cancellation/concurrency contracts.

The scan gate parses govulncheck's JSON protocol explicitly: JSON output alone must not be treated as an exit-code-only reachability verdict. Module/package findings are retained for review; symbol-reachable findings block release, conservatively regardless of severity. Vulnerability DB snapshots change, so retain tool/database timestamps with the scan artifact. Private disclosure activation and an independent security review remain owner/reviewer gates; SECURITY.md records the process.
