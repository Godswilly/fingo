# FinGo Error Taxonomy and Mapping (Week 1)

This document defines the standard error model for FinGo.

## Goals

- Preserve root cause for debugging (`errors.Is`/`errors.As` works).
- Classify failures by layer and code.
- Keep transport mapping deterministic.
- Prevent leaking infrastructure internals to API clients.

## Error Shape

Use `internal/errs.Error`:

- `Layer`: where failure originated (`domain`, `application`, `infrastructure`)
- `Code`: business/behavioral category (`invalid_argument`, `conflict`, `timeout`, etc.)
- `Op`: operation name (`transfer.Create`, `postgres.Exec`)
- `Message`: safe summary message
- `Err`: wrapped root error (optional)

## Layer Rules

- `domain`: invariant and business rule failures only.
- `application`: orchestration/use-case failures (workflow, idempotency, policy).
- `infrastructure`: external system failures (DB, network, broker, I/O).

Never return raw infrastructure errors directly from transport handlers.

## Standard Codes

- `invalid_argument`
- `not_found`
- `conflict`
- `invariant_violation`
- `unauthenticated`
- `permission_denied`
- `timeout`
- `unavailable`
- `internal`

## Construction Helpers

Use constructors from `internal/errs`:

- `errs.Domain(...)`
- `errs.Application(...)`
- `errs.Infrastructure(...)`

Example:

```go
return errs.Domain(
    errs.CodeInvariantViolation,
    "ledger.Post",
    "posting set must balance",
    nil,
)
```

## Mapping Guidelines

### HTTP status mapping

- `invalid_argument` -> `400`
- `invariant_violation` -> `422`
- `unauthenticated` -> `401`
- `permission_denied` -> `403`
- `not_found` -> `404`
- `conflict` -> `409`
- `timeout` -> `504`
- `unavailable` -> `503`
- `internal` or unknown -> `500`

Use `errs.HTTPStatus(err)` for consistent behavior.

### gRPC status mapping guideline

- `invalid_argument` -> `InvalidArgument`
- `invariant_violation` -> `FailedPrecondition`
- `unauthenticated` -> `Unauthenticated`
- `permission_denied` -> `PermissionDenied`
- `not_found` -> `NotFound`
- `conflict` -> `AlreadyExists` (or `Aborted` based on operation semantics)
- `timeout` -> `DeadlineExceeded`
- `unavailable` -> `Unavailable`
- `internal` or unknown -> `Internal`

## Usage Pattern by Layer

- Domain packages return domain-coded errors for invariant breaches.
- Application services may wrap domain/infrastructure errors with operation context.
- Infrastructure adapters map external client errors into infrastructure-coded errors.
- Transport handlers convert coded errors into protocol status without exposing internals.

## Logging Rules

- Log full error chain internally with operation context.
- Return only safe `Message` to clients.
- Never include secrets, DSNs, tokens, or raw SQL in client-facing messages.
