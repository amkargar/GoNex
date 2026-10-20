# Server output and structured logging

[Project home](../README.md) · [Documentation index](README.md)

Gonex uses application-scoped `log/slog` loggers for lifecycle and HTTP completion events. It does not replace the process-wide default logger.

## Run an application

From a CLI-generated app:

```sh
go tool gonex run
```

Readable development output shows the application name, resolved Gonex version, environment, listening address and clickable local URL. Wildcard listeners (`0.0.0.0` or `::`) display a localhost URL with the actual port; this is a local convenience, not a public deployment URL. OpenAPI and optional `/docs` URLs are shown only when enabled, along with registered version-specific documents. HTTP-disabled apps show no HTTP URLs. The startup event is emitted after resources start and the listener binds, never on a failed bind; dependency readiness is still reported separately by `/readyz`.

Ctrl+C triggers graceful shutdown in the generated executable. Lifecycle events report startup failure, stopping, shutdown failure and successful stop.

## Configuration

The generated `config.Load()` reads these variables:

| Variable | Default | Meaning |
| --- | --- | --- |
| `APP_NAME` | `Gonex` | Application name in startup output |
| `GONEX_ENV` | `development` | `production` selects JSON for the auto format |
| `GONEX_LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `GONEX_LOG_FORMAT` | `auto` | `auto`, `pretty`, `text`, `json` |
| `GONEX_LOG_COLOR` | `auto` | Pretty output: `auto`, `always`, `never` |

Auto color is disabled when stderr is not a terminal, `TERM=dumb`, or `NO_COLOR` is set. JSON is one object per line. Logs go to stderr, suitable for container/platform log collection. This does not implement file rotation, remote storage or an external collector. Environment variables are read by the generated app configuration, not implicitly by the core library. Export them in your process environment; `.env` is not loaded automatically.

Manual consumers configure `config.Config` with `Name`, `Environment`, `LogLevel`, `LogFormat` and `LogColor`. A caller-supplied `Logger *slog.Logger` takes precedence. `DisableRequestLogging` and `DisableStartupBanner` opt out of completion events and the startup summary respectively.

## Request events

| Field | Meaning |
| --- | --- |
| `time`, `level`, `msg` | slog timestamp, severity and event message |
| `service`, `environment` | Application identity and environment from config |
| `trace_id` | W3C trace identifier shared with unary RPC calls |
| `request_id` | Fresh server-generated identifier, also returned as `X-Request-ID` |
| `method`, `path` | Standard HTTP method (other tokens use `OTHER`) and registered route pattern; unknown paths use `unmatched` |
| `status`, `duration_ms` | Response status and elapsed processing milliseconds |
| `error` | Safe framework error code or `handler panic`, when applicable |

Successful responses use INFO; 4xx use WARN; 5xx and panics use ERROR. One completion event is emitted per request, including recovered panics. Additional business log events are separate. Filtering by log level may suppress lower-severity events. Existing metrics/trace middleware can still be installed separately.

The default request event excludes request/response bodies, headers, query strings, raw path parameter values, arbitrary error strings and panic payloads. Application code remains responsible for the fields it explicitly logs.

## Log from a service

```go
observability.Logger(ctx, nil).InfoContext(ctx,
    "user created",
    "module", "users",
    "user_id", user.ID,
)
```

Import `github.com/amkargar/gonex/observability`. Pass the request context into services; the logger carries its request ID. Outside an HTTP request, supply the application logger as the fallback (`observability.Logger(ctx, application.Logger())`). The `module` field is explicit business context, not inferred from the URL. `observability.RequestID(ctx)` reads the current ID directly.

For a custom destination, construct a logger with `observability.NewLogger(observability.LogOptions{Output: writer, Format: "json", Level: "INFO"})` and pass it through `config.Config.Logger`. Keep asynchronous tasks' cancellation and log context deliberate.

Lifecycle failures use safe startup_failed/resource_failed/shutdown_failed codes in default logs. Detailed causes are returned to the caller for deliberate private handling; do not log credential-bearing causes indiscriminately.
