# API decisions

Module path: `github.com/amkargar/gonex`. Toolchain: Go 1.27.1, the installed repository baseline. Gin 1.12.0 will be pinned when Phase 1 introduces it.

The following are frozen contracts for Phase 1, not implemented APIs in Phase 0:

- `gonex.New(config.Config) (*app.App, error)` validates settings without binding ports.
- `Module.Register(*Registrar) error`; `App.Register(...Module) error` registers explicitly in argument order, atomically per call.
- `Registrar.Handle(method, path string, handlers ...gin.HandlerFunc) error` registers full paths. Middleware is passed in order with each route; no mutable global groups.
- Route identity is `(uppercase method, full path)`. Duplicate and incompatible wildcard routes fail before serving. `/healthz` is owned by the runtime.
- `Registrar.Manage(Resource) error`, where Resource has `Start(context.Context) error` and `Close(context.Context) error`, provides explicit resource ownership.
- `App.Start(context.Context) error` starts resources and binds HTTP synchronously. `App.Run(context.Context) error` waits for cancellation or server failure and shuts down. `App.Shutdown(context.Context) error` is idempotent. A stopped instance cannot restart.
- States: created, registered, starting, running, stopping, stopped. Registration freezes at startup. A registration failure leaves the application unchanged.
- Configuration supplies address, read/write/idle and shutdown timeouts. Zero values get documented defaults; negative durations fail.

Future phases choose their concrete public APIs when introduced; no placeholder packages are created.
