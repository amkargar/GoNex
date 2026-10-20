# Compatibility and upgrades

[Project home](../README.md) · [Documentation index](README.md)

## Contents

- [Module version policy](#module-version-policy)
- [Upgrade checklist](#upgrade-checklist)
- [HTTP compatibility](#http-compatibility)
- [Protobuf compatibility](#protobuf-compatibility)
- [Strict DTO changes](#strict-dto-changes)
- [Template compatibility](#template-compatibility)

## Module version policy

goNex uses semantic module versions. Until v1, minor versions may change APIs and templates; patch versions should preserve behavior except documented bug/security fixes. Review the changelog, pin the framework version and upgrade in a branch. No automatic migration of handwritten code is provided.

## Upgrade checklist

1. Update the framework requirement and run `go mod tidy` independently in each service.
2. Review DTO validation, lifecycle behavior and generated registries before regenerating. `.gonex.json` format 1 records hashes; generators refuse changed owned files and incompatible layouts. Resolve conflicts explicitly rather than deleting metadata.
3. Run tests, vet, build, migration checks and contract generation/checks. Database migrations are ordered immutable files: append a migration instead of editing applied SQL. Take deployment-specific backups before applying schema changes.
4. Export every supported HTTP version and compare its release baseline. Deploy consumers/providers in a compatible order before retiring old contracts.

## HTTP compatibility

HTTP compatibility includes methods/paths, operation IDs, parameter locations/types/requiredness, body schemas, response status/schema and security requirements. `gonex openapi validate --file current.json --against baseline.json` performs a conservative comparison: additions are allowed but changes to existing contracts are rejected, including some harmless descriptive edits. Review such changes explicitly; do not silently replace the baseline. Keep separate baselines for each HTTP major. See [versioning](versioning.md).

## Protobuf compatibility

Protobuf packages encode the API major (`identity.v1`). Keep field numbers/types, service/method identities and request/response types compatible. Run both `gonex grpc check` and Buf breaking against the released descriptor baseline. Removing/repurposing fields or methods requires a new version and a consumer migration. The internal checker is deliberately conservative; Buf is an additional release gate.

## Strict DTO changes

This implementation tightens early-phase behavior: duplicate JSON keys and unsupported validation tags now fail; nested request objects require pointers; custom JSON encodings require explicit transport DTOs. A value body wrapper is already required and should not carry `validate:"required"`; use a pointer if the body's explicit presence is validated. Deadline failures have a documented 504 response. These changes prevent runtime/schema disagreement; update DTOs rather than weakening exported contracts.

## Template compatibility

Templates do not infer business mappings. Older module layouts require the injectable service factory before automatic gRPC enable. Existing handwritten adapters are retained when versions are added; `--policy replace` is an explicit legacy-route registration change.

## Production-readiness hardening upgrade

The minimum Go patch is now 1.27.2 because the vulnerability database reported reachable standard-library issues in 1.27.1. Re-resolve the pinned gRPC, pgx, QUIC and x/* dependency updates with go mod tidy in every module. Keep source contracts independently pinned. Existing handwritten apps do not automatically acquire new generated config fields; logging/telemetry is configured explicitly.

Metrics snapshots add cumulative duration buckets and enforce a 128-operation budget. HTTP request logging now adds service/environment/trace IDs and the traceparent response header. Clients should treat additional operational headers/JSON fields as additive. Applications constructing exported config/measurement structs should use keyed fields. There is no automatic model CRUD or template renderer in this upgrade.

Startup now permits bounded concurrent readiness/shutdown while resources start, and owns the listener until it is explicitly closed. Resource callbacks must honor cancellation. When upgrading a running service, drain the old binary and run contract/DB checks before promotion; binary rollback needs backward-compatible database changes.
