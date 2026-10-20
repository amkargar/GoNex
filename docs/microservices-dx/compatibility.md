# Compatibility and ownership

Existing main.go/config/modules paths remain supported. No new cmd/server/internal/bootstrap profile or automatic path migration is introduced. Re-generation refuses caller-owned destinations before writes; dry-run changes no project files and runs no codegen plugins.

New schema-1 gonex.yaml is caller-editable local configuration. It rejects unknown/duplicate keys, duplicate listener ports, path escape, symlink service paths and non-loopback endpoints. It does not replace go.mod versions or business bootstrap. Declared dependency cycles warn rather than banning peer topology. Older workspaces retain run/build/check without a catalog; add a validated catalog to opt into dev/doctor.

Existing create service NAME retains HTTP defaults; explicit --http, --grpc, both, or --transport none select listeners. Module adapters are separate; HTTP API versions remain opt-in. Conflicting/irrelevant flags are rejected before writes.

New clients default to compile-time skeletons. This deliberately exposes unfinished mapping/wiring. --profile raw retains the previous typed-client/manual-bootstrap workflow; existing client files are never rewritten. --profile example is limited to the exact descriptor-verified demonstration contract, with --rpc Service.Method resolving ambiguity.

v1 keeps its previous names. v2 gets separate files/settings/types. Share a managed pool/config explicitly using generated WithConfig constructors when multiple versions/modules consume one target; do not call multiple convenience helpers for the same security profile. No global singleton or locator exists.

Edit contracts/proto; pb.go is generated. grpc generate stages outputs atomically, protects handwritten destinations and checks baseline.binpb when supplied. grpc generate --check regenerates in memory and reports drift without project writes. A placeholder v0.1.0 is not a released baseline; published baselines remain deliberate inputs. Buf/Go API gates stay enabled.

Publish actual contracts through normal module hosting or an immutable proxy before isolated deployment. Services build with GOWORK=off and no replacement. CI's disposable file proxy proves that mechanism; it is not a public release or required runtime infrastructure.

Existing bounded RPC deadlines, TLS/mTLS, health/readiness, drain and disabled automatic retries remain intact. Operators, meshes, dynamic discovery, the optional graph command and new path-template profiles are deferred.
