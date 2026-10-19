# Compatibility and upgrades

Gonex uses semantic module versions. Until v1, minor versions may change APIs and templates; patch versions should preserve behavior except documented bug/security fixes. Review the changelog, pin the framework version and upgrade in a branch. No automatic migration of handwritten code is provided.

1. Update the framework requirement and run `go mod tidy` independently in each service.
2. Review DTO validation, lifecycle behavior and generated registries before regenerating. `.gonex.json` format 1 records hashes; generators refuse changed owned files and incompatible layouts. Resolve conflicts explicitly rather than deleting metadata.
3. Run tests, vet, build, migration checks and contract generation/checks. Database migrations are ordered immutable files: append a migration instead of editing applied SQL. Take deployment-specific backups before applying schema changes.
4. Export every supported HTTP version and compare its release baseline. Deploy consumers/providers in a compatible order before retiring old contracts.

HTTP compatibility includes methods/paths, operation IDs, parameter locations/types/requiredness, body schemas, response status/schema and security requirements. `gonex openapi validate --file current.json --against baseline.json` performs a conservative comparison: additions are allowed but changes to existing contracts are rejected, including some harmless descriptive edits. Review such changes explicitly; do not silently replace the baseline. Keep separate baselines for each HTTP major. See [versioning](versioning.md).

Protobuf packages encode the API major (`identity.v1`). Keep field numbers/types, service/method identities and request/response types compatible. Run both `gonex grpc check` and Buf breaking against the released descriptor baseline. Removing/repurposing fields or methods requires a new version and a consumer migration. The internal checker is deliberately conservative; Buf is an additional release gate.

This implementation tightens early-phase behavior: duplicate JSON keys and unsupported validation tags now fail; nested request objects require pointers; custom JSON encodings require explicit transport DTOs. A value body wrapper is already required and should not carry `validate:"required"`; use a pointer if the body's explicit presence is validated. Deadline failures have a documented 504 response. These changes prevent runtime/schema disagreement; update DTOs rather than weakening exported contracts.

Templates do not infer business mappings. Older module layouts require the injectable service factory before automatic gRPC enable. Existing handwritten adapters are retained when versions are added; `--policy replace` is an explicit legacy-route registration change.
