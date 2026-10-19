# Databases and migrations

[Project home](../README.md) · [Documentation index](README.md)

## Contents

- [Configure a database resource](#configure-a-database-resource)
- [Keep repositories at the boundary](#keep-repositories-at-the-boundary)
- [Apply immutable migrations](#apply-immutable-migrations)
- [Run the SQLite example](#run-the-sqlite-example)
- [Verify database behavior](#verify-database-behavior)

## Configure a database resource

`orm.New(orm.Options{Dialect:"sqlite", DSNEnv:"DATABASE_URL"})` constructs a resource without pinging. Register it with `Registrar.Manage`; startup opens/pings and shutdown closes its SQL pool. `orm.Open(ctx, options)` opens immediately for explicit tools/tests. Never log DSNs. Driver errors from opening/pinging are redacted. Pool limits, idle time and lifetime are configurable. The initial verified dialect is real file-backed SQLite using ncruces/gormlite (CGO-free); PostgreSQL uses GORM's official driver.

## Keep repositories at the boundary

Services define local repository interfaces; GORM adapters map values and not-found/unique errors. `orm.Transaction` accepts a GORM callback only at the adapter boundary. A service needing atomic multi-write behavior should define a small local transaction callback interface implemented by that adapter.

## Apply immutable migrations

Migrations are immutable UTF-8 SQL files named `000001_description.up.sql`. Use `migration.New(pool, dialect, files)` with an embedded or directory `fs.FS`, then explicitly call `Up(ctx)` or `Status(ctx)`. SQL must be transaction-compatible; do not put BEGIN/COMMIT, nontransactional DDL or connection-level PRAGMAs inside a migration. The runner uses one transaction for the full pending batch, a version/checksum history table, SQLite BEGIN IMMEDIATE via serializable isolation, and PostgreSQL transaction advisory locks. Concurrent runners cannot apply the same migration twice. Changes to applied contents/names, missing versions and out-of-order history fail clearly. Repeating an unchanged migration set is a no-op. Status initializes an empty history table if needed.

No AutoMigrate or implicit application-start schema changes. Down migrations are deliberately absent from the example because dropping persisted users is destructive. Forward corrections use new versions.

## Run the SQLite example

PowerShell example:

```powershell
$env:DATABASE_DIALECT='sqlite'
$env:DATABASE_URL='file:users.db?_pragma=busy_timeout(5000)'
go run ./examples/basic migrate
go run ./examples/basic
```

POST /users persists rows; subsequent GET /users/{id} works across application restarts. Without DATABASE_URL the example uses its in-memory repository, and HTTP-only apps import no persistence package.

## Verify database behavior

`go test ./...` always tests file-backed SQLite CRUD, transactions, migration rollback/checksums/concurrency and lifecycle cleanup. PostgreSQL integration is explicitly skipped unless `GONEX_TEST_POSTGRES_DSN` identifies an **empty disposable test database**; it is never inferred from production DATABASE_URL. PostgreSQL credentials and deployment-specific behavior require that integration environment.
