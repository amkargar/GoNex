# Microservices DX baseline

Assessed source: fb44296864d8798b86651854465cf10f3b7a677e (2026-10-20). Existing service layout (main.go, config/, modules/) is retained. No automatic path migration or new runtime abstraction is necessary.

Existing CLI supports microservice workspaces, independent service modules, typed protobuf clients, optional HTTP/grpc module adapters, explicit TLS/development insecurity and safe transactional generator plans. Generated contracts start at an explicitly unpublished workspace-local v0.1.0; local go.work resolves that placeholder, but deployment requires an actual published contracts module version. Source fixtures with replaces are not publication evidence.

Friction: no dev supervisor or doctor, no shared catalog, service transport flags unavailable, ports allocated by service count, client structs compile without their business mapping/wiring being completed, and grpc generate has no drift-check flag. Client loading/security/timeout settings are repeated manually in application factories. Existing TLS/mTLS, compatibility, race, shutdown and end-to-end generation tests provide the preserved baseline.

Golden fixture: internal/scaffold/testdata/project.golden.json. Existing generated projects are not rewritten. The new client example is deliberately limited to Gonex's exact empty-request/string-message demonstration contract; arbitrary business mappings remain caller-owned.
