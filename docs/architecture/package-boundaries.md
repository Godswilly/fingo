# FinGo Package Boundaries (Week 1)

This document defines ownership rules for `cmd`, `internal`, and `pkg`.

## `cmd/`

Purpose: executable entrypoints only.

- `cmd/fingo-api`: API server bootstrap
- `cmd/fingo-worker`: asynchronous/background workers bootstrap
- `cmd/fingo-migrate`: migration runner bootstrap

Rules:

- Keep `main.go` thin: parse config, wire dependencies, start process.
- Do not place domain logic in `cmd`.

## `internal/`

Purpose: private application code.

- `internal/domain`: domain entities/value objects/invariants
- `internal/app`: use-cases coordinating domain + adapters
- `internal/transport`: adapters for inbound protocols (gRPC/HTTP)
- `internal/database`: adapters for data stores (Postgres)
- `internal/messaging`: adapters for NATS event publishing/consuming
- `internal/observability`: logging/metrics/tracing setup
- `internal/config`: config parsing + validation
- `internal/errs`: internal shared error types
- `internal/platform`: dependency injection/bootstrap wiring

Rules:

- Domain code must not import transport/database/messaging packages.
- Adapters depend inward on domain/app; never the reverse.
- Cross-package dependencies should follow: `cmd -> platform/app -> domain`.

## `pkg/`

Purpose: reusable libraries intentionally safe for external consumption.

Current candidates:

- `pkg/money`: money-safe utility types/helpers
- `pkg/xcontext`: context helper utilities

Rules:

- Default to `internal/`; only move to `pkg/` when API stability is intentional.
- Public package APIs require docs and tests.
