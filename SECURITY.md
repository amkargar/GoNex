# Security policy

Gonex is pre-release. Security fixes target the current reviewed development branch; no historical release line or response-time SLA is promised yet. Use the patched Go baseline in go.mod and review dependency scan results before deployment.

## Reporting

Use GitHub's private **Security → Report a vulnerability** for this repository when enabled:
https://github.com/amkargar/gonex/security/advisories/new

Private vulnerability reporting must be enabled by a repository administrator. Its availability has not been confirmed by the current integration. Until enabled, open an issue requesting a private reporting channel **without exploit details, credentials or sensitive data**; wait for a maintainer-provided private channel before sharing them. Do not put undisclosed vulnerabilities in public pull requests.

Include the affected commit/version, environment, minimal reproduction, impact and suggested mitigation. Maintainers should acknowledge, reproduce, assess reachability, prepare a regression test/fix, coordinate disclosure and publish affected/fixed versions. Response times depend on maintainer availability; this project does not promise a 24/7 response service.

## Release gates

Reachable govulncheck findings fail the security gate; findings are not waived merely because a package is optional. Gitleaks scans repository history with redacted reports. Fuzzing and negative boundary tests supplement scanning; none establishes absence of vulnerabilities. See docs/security-threat-model.md and docs/production-readiness/final-report.md for remaining release blockers.
