# Modules and lifecycle

[Project home](../README.md) · [Documentation index](README.md)

## Contents

- [Register a module](#register-a-module)
- [Keep business logic independent](#keep-business-logic-independent)
- [Share services across adapters](#share-services-across-adapters)
- [Manage resources](#manage-resources)
- [Own background work](#own-background-work)

## Register a module

A module implements `Register(*app.Registrar) error`. Registration declares routes, managed resources and named readiness checks; it should not open listeners or databases. Register related modules together when their declarations must succeed atomically. Registration seals before startup, and retained registrars cannot mutate a running app.

## Keep business logic independent

Keep business services in `modules/NAME/service.go`. They accept context and domain values and depend on small interfaces owned by their consumer. HTTP schemas/handlers, Protobuf mappers and GORM adapters translate at the boundary. Avoid a shared repository that exposes another service's database.

## Share services across adapters

Generated modules expose an injectable service and `NewModuleService`; generated registries create one service for all its adapters. Change business logic in handwritten files. Tool-owned registries are hash-checked and must be reconciled explicitly if manually edited.

## Manage resources

Use `Registrar.Manage` for resources implementing `Start(context.Context) error` and `Close(context.Context) error`. Dependencies register before their consumers: database, client pool, then server. Startup failure closes resources already started; shutdown drains HTTP and closes resources in reverse order. Resources can expose an `Errors() <-chan error` channel for asynchronous failure; `App.Run` then stops the application and returns an error. Resources must honor their lifecycle contexts.

## Own background work

`Start` is explicit, `Run` waits for cancellation/failure, and `Shutdown` is idempotent. A stopped application cannot restart. Do not start unmanaged goroutines from registration: own their cancellation and join them from a resource's Close method. See [operations](operations.md) for health checks and shutdown behavior.
