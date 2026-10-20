# Typed HTTP and OpenAPI

[Project home](../README.md) · [Documentation index](README.md)

## Contents

- [Register typed endpoints](#register-typed-endpoints)
- [Bind and validate requests](#bind-and-validate-requests)
- [Handle failures and limits](#handle-failures-and-limits)
- [Describe endpoint metadata](#describe-endpoint-metadata)
- [Serve and export documents](#serve-and-export-documents)
- [Supported DTO schemas](#supported-dto-schemas)
- [Freeze and compare contracts](#freeze-and-compare-contracts)

## Register typed endpoints

Declare routes together and pass typed handler methods directly. Request and response types are inferred from each handler:

```go
func Register(r transport.EndpointRegistrar, actions Actions) error {
    h := Handler{Actions: actions}
    return transport.Routes(r,
        transport.Post("/users", h.Create).Name("createUser").Errors(409),
        transport.Get("/users/:id", h.Get).Name("getUser").Errors(404),
    )
}
```

Import `transport "github.com/amkargar/gonex/http"`. A handler has the signature `func(context.Context, Request) (Response, error)`; named structs in `schemas.go` declare the contract. Give every route a unique `.Name(...)` for its OpenAPI operation ID.

`Get`, `Put`, `Patch`, `Head` and `Options` default to 200; `Post` defaults to 201 and `Delete` to 204. Override with `.Status(...)`. A 204 endpoint should return `NoBody`.

Optional metadata stays next to the route: `.Summary(...)`, `.Description(...)`, `.Tags(...)`, `.Errors(...)`, `.Use(...)`, `.Security(requirements, schemes)`, `.MaxBodyBytes(...)` and `.Examples(request, response)`. Middleware runs before binding and validation. Versioned registrars work with the same route declarations.

`Routes` uses the existing typed registration pipeline: validation, binding, request limits, error mapping and OpenAPI remain identical. Return its error from `Module.Register`; the application commits module registration only after all modules succeed. Reusing a declaration to register multiple paths is discouraged; define each route once during startup.

The lower-level `http.Register(registrar, http.Endpoint[Request, Response]{...})` API remains supported for existing applications. Business services return plain Go values; adapters map those into transport schemas.

## Bind and validate requests

Each request wrapper field declares exactly one origin: `path:"id"`, `query:"page"`, `header:"X-Token"` or `body:"true"`. There may be only one body field. Parameters are scalar or pointers to scalars. Path placeholders must match field bindings. Body schemas use standard `json` tags and `validate` tags. Required, min/max, email, nested structs and slice `dive` validation are supported. Use pointers to distinguish absent numeric/boolean values from zero/false; `required` on a pointer requires presence, while `required` on a scalar rejects its zero value.

| Origin | Example tag | Constraint |
|---|---|---|
| Path | `path:"id"` | Required; must match the route placeholder |
| Query | `query:"page"` | Scalar or pointer to a scalar |
| Header | `header:"X-Token"` | Scalar or pointer; duplicates are rejected |
| Body | `body:"true"` | One JSON body field per request wrapper |

## Handle failures and limits

Malformed JSON, missing required parameters and unsupported content types return 400. Unknown or incorrectly cased JSON fields, duplicate object keys, trailing values, duplicate query/header values and null nonnullable bodies are rejected. Validation failures return 422 with field paths and codes. Request bodies default to 1 MiB; set `MaxBodyBytes` per endpoint. Application errors must be declared in `Errors` to expose their mapped status/message; undeclared/internal errors return a redacted 500. Context deadline failures return a documented 504. 204 responses have no body.

## Describe endpoint metadata

Optional summary, description, tags, request/response examples, security requirements/schemes and middleware live on the same endpoint contract. Middleware must return documented response shapes/statuses. Low-level `Registrar.Handle` is an escape hatch for raw Gin routes and is not projected into OpenAPI; public application endpoints should use typed registration.

## Serve and export documents

`/openapi.json` is enabled by default; `DisableOpenAPI` removes it in production. `/docs` is an opt-in local document link (`DocsEnabled`), without externally loaded assets. Runtime-owned paths reject conflicts. The specification is built and validated once during startup, not per request. `App.OpenAPI()` explicitly exports without starting a listener.

## Supported DTO schemas

Schemas support named/nested/recursive structs, pointers/nullability, string-keyed maps and slices; component names include a stable package hash. Input requiredness follows validation, output requiredness follows JSON omission. Nested input objects require pointers (except time.Time) so presence has an explicit representation. A value body wrapper is already required: omit its redundant required validation tag. Anonymous embedded fields, byte slices, non-string map keys, custom JSON encodings and unsupported JSON tag options require explicit DTOs. Supported validation tags are required, omitempty, min, max, email, dive and -; unsupported or contradictory constraints fail registration. The official OpenAPI Initiative 3.1 schema is vendored with its source URL; santhosh-tekuri/jsonschema v6 validates generated documents offline.

## Freeze and compare contracts

Contracts and handler slices are detached from caller-owned metadata at registration. Security requirements must reference declared schemes. Export a version with `gonex openapi generate --http-version v1 --out v1.json`; compare it with a released baseline using `gonex openapi validate --file v1.json --against baseline.json`. Compatibility checking is conservative; see [upgrade policy](upgrade.md).
