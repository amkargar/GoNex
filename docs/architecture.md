# Architecture

Developer features live in `modules/`. Their services use plain Go values and small local dependency interfaces. HTTP adapters own `schemas.go`; repositories implement those interfaces using GORM. No service imports Gin, GORM, gRPC, Protobuf or HTTP DTOs.

Module registration is explicit and ordered. A module receives registration capabilities, never ownership of a listener. The application owns listeners and resources. Startup failures close previously started resources in reverse order. Shutdown is bounded and repeatable.

HTTP is unversioned by default. OpenAPI projects registered typed endpoints. Versioned HTTP adapters share core services; legacy routes remain unless replacement is explicit. gRPC is optional; all Protobuf packages are versioned. Contracts are a separate module.

Generators own only marked generated registries. They plan and validate all writes first, refuse existing handwritten destinations and support dry runs. No runtime source scanning or dependency injection container.
