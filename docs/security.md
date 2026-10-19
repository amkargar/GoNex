# Security configuration

Authentication is application-owned. `security.Authenticate(verifier)` delegates credential extraction and verification, requires a nonempty subject and returns a generic 401 on failure. `security.Current(ctx)` reads the principal. `security.Authorize(predicate)` returns 403 for a rejected authenticated principal. Install authentication before authorization. Declare matching endpoint error statuses and OpenAPI security schemes; metadata alone does not enforce authentication. No JWT/session scheme is assumed.

No CORS middleware is enabled automatically. `security.CORS` accepts explicit HTTP(S) origins, methods and header allowlists, validates configuration and denies unknown origins/preflights. Wildcards are rejected. Credentials are opt-in; preflight caching is limited to 24 hours. CORS does not replace authentication or CSRF defenses for cookie-based applications.

Typed bodies default to 1 MiB and reject unknown fields, duplicate keys, trailing JSON, incorrect field case and null nonnullable values. Parameter duplicates and nonfinite floats are rejected. Use pointer DTO fields for optional/presence-sensitive values and pointer nested input objects. Set `MaxBodyBytes` for an endpoint and `http.Deadline` for cooperative request deadlines. Server defaults are read 10s, write 30s, idle 60s and shutdown 10s. Configure ingress and application budgets together.

The HTTP listener is plain HTTP intended for a configured TLS ingress or private local deployment. gRPC requires TLS 1.2 or newer; loading helpers support CA verification and mTLS. Plaintext requires explicit `DevelopmentInsecure`; clients reject skipped certificate verification. No automatic RPC retries are enabled. Managed clients apply a five-second default unary/stream deadline, bounded by an earlier caller deadline.

Internal errors, panic payloads, readiness failures and database startup credentials are redacted at framework boundaries. Recovery does not dump request headers or panic values. Application logs, SQL logging, explicit public error messages and caller-supplied interceptors remain the application's responsibility. Trace identifiers are correlation data, never authentication credentials.

OpenAPI is enabled by default: set `DisableOpenAPI` when it should not be public. `/docs` is opt-in and loads no external assets. Trusted proxies are disabled unless configured outside the framework's router. Metrics exposure is opt-in in the example. Load secrets from deployment configuration; do not commit credentials.
