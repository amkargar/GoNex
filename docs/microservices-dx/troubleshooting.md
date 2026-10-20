# Microservice development troubleshooting

| Diagnostic | Action |
| --- | --- |
| mapping_todo / GONEX_TODO | Implement the local interface, descriptor-to-domain field/error mapping and factory injection. Remove the compile-time marker deliberately. |
| wiring_missing | Call NewProviderForModule once from the caller-owned factory. Static analysis is guidance; test the actual mapped RPC. Raw clients use manual bootstrap. |
| server_tls_missing / client_tls_missing | Supply TLS material or explicitly select --dev-insecure for loopback development. No production plaintext default is generated. |
| invalid_server_tls / invalid_client_tls | Verify certificate, key, CA, server name and expiration. Doctor parses material; a live handshake validates identity/connectivity. Credential contents/raw loader errors are never printed. |
| security_flags_conflict | Unset insecure flags when TLS material is configured. |
| production_plaintext | Remove development insecurity; GONEX_ENV=production cannot use it through dev. |
| port_unavailable | Stop the conflicting process or change the service-scoped address/catalog. The runner never kills unrelated processes. Check/start bind races remain possible and child failure is reported. |
| Shared listener address is ambiguous | Use GONEX_SERVICES_NAME_HTTP_ADDRESS/GRPC_ADDRESS or select one --service. |
| contracts_stale / contracts_not_generated | Explicitly run grpc generate or dev --generate. grpc generate --check checks artifact drift without project writes; explicit generation may prepare/download pinned plugins. |
| go_version / go_missing | Select Go 1.27.2 or newer on PATH; dev never downloads a toolchain. |
| Missing module/checksum | Prepare dependencies explicitly. First local use tidies the root CLI module and retains go.work. For isolated service tidy/build, publish/select actual contracts and set GOWORK=off. Never disguise an unpublished module with a production replace. |
| Orders returns 503 | Check Identity target, TLS and readiness. RPC deadlines remain bounded; do not hide mutating failures with automatic retries. |
| Worker exits immediately | Any unexpected child exit, including status zero, stops siblings and returns non-zero. |

No dotenv loading, implicit discovery, required database or central runtime is added. Unix uses process groups. Windows uses Job Objects, attempts console break and uses bounded termination if a console signal is unavailable. Forced termination does not guarantee application callbacks finished cleanup.
