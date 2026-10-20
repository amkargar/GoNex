# Development pages and DEBUG

Debug pages are opt-in and disabled by default.

## Configure a generated application

Set these environment variables in the process running your application:

```sh
# Linux / macOS
DEBUG=true go tool gonex run
```

```powershell
# PowerShell
$env:DEBUG = "true"
go tool gonex run

# Turn debugging off
$env:DEBUG = "false"
```

The generated `config.Load()` maps the literal `DEBUG=true` to
`config.Config.Debug`. Unset, `false` and other values disable debugging.
The core library stays explicit: manual applications set `config.Config{Debug: true}`.
Environment variables are read by the generated app, not automatically by the
library. A `.env.example` is a reference; Gonex does not automatically load it.

## Welcome page

With `DEBUG=true`, visiting `GET /` shows **Welcome to Gonex**, the Gonex duck,
and links to the website and documentation. The image is embedded in the
executable, so this page works offline and does not require a static directory.
An OpenAPI link appears only when OpenAPI is enabled.

Your own matching route always wins, including a custom root or catch-all route.
With `DEBUG=false`, Gonex does not install the welcome page or debug image.
Normal application routes continue to work.

## Browser error pages

A browser request accepting `text/html` receives an HTML development page for
uncommitted error responses, including:

- 404: request path and registered routes.
- 405: a method that is not supported by a registered route.
- Binding/validation errors: status and validation information.
- Returned handler errors: original error message, type and stack.
- Panics: panic value and stack trace.

The page includes method, query-free path, matched route and request ID. HTML is
escaped. Request bodies, headers, cookies, query strings and environment
variables are not dumped. Error strings and stacks can contain sensitive details,
so keep `DEBUG=false` on public deployments.

Requests for JSON (or without an explicit HTML preference) keep their normal API
response shape and generic internal errors, even with debug enabled.
This preserves typed OpenAPI contracts for API clients.

Already committed responses, streams, hijacked connections and error responses
over 64 KiB are not rewritten. Successful responses and streams pass through
immediately. Debug HTML is never cached and contains no external scripts.

## Disable debugging in production

```sh
DEBUG=false
GONEX_ENV=production
```

`GONEX_ENV` controls logging defaults; it does not implicitly turn debug on or off.
The two switches remain explicit. Restart the application after changing them.
