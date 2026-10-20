# Production promotion checklist

Do not infer completion from the presence of this checklist. Link evidence per deployment/release.

- [ ] Exact reviewed commit/tag, pinned dependencies, patched Go toolchain and artifact digest recorded.
- [ ] Required quality, security, contract, race and real supported-database checks passed.
- [ ] Required-status/PR protection is enforced and a failing probe was blocked.
- [ ] Private disclosure route and independent security review verified.
- [ ] Public tagged install and prior-version upgrade/rollback rehearsal completed where applicable.
- [ ] Application authn/authz, CORS/CSRF decisions, secrets and TLS reviewed.
- [ ] Migrations, backups/restores and forward recovery rehearsed; binary rollback is schema-compatible.
- [ ] Health/readiness, request deadlines, log privacy and operational endpoint access verified.
- [ ] PostgreSQL/RPC failures and shutdown/drain behavior match the documented runbook.
- [ ] Representative load/soak, latency tails, errors and resource trends measured on deployment hardware.
- [ ] Operator troubleshooting and an unfamiliar developer's onboarding rehearsal completed.
- [ ] Support scope and residual limitations explicitly accepted by the release owner.

Until the [final report](production-readiness/final-report.md) closes its blockers, the project must not describe itself as a fully production-ready or historically mature release.
