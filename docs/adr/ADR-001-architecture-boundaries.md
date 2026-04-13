# ADR-001: FinGo Architecture Boundaries and Dependency Rules

- Status: Accepted
- Date: 2026-04-13
- Decision makers: FinGo core maintainer

## Context

FinGo is a correctness-first ledger system. To prevent accidental coupling between transport, persistence, and business rules, we need explicit package boundaries and dependency rules from Week 1.

Without strict boundaries:

- domain invariants can leak into handlers and repositories
- infrastructure concerns can pollute business logic
- tests become brittle and expensive
- refactors become high risk

## Decision

Adopt a layered structure with inward dependencies:

- `cmd/*` for process entrypoints only
- `internal/domain/*` for business entities and invariants
- `internal/app/*` for use-case orchestration
- `internal/transport/*` for protocol adapters (HTTP/gRPC)
- `internal/persistence/*` for storage adapters (Postgres/migrations)
- `internal/messaging/*` for event adapters (NATS)
- `internal/config`, `internal/errs`, `internal/observability`, `internal/platform` for cross-cutting concerns

Dependency rule:

- outer layers may depend inward
- inner layers must not depend outward
- specifically, domain must not import transport/persistence/messaging

## Alternatives considered

1. Feature-sliced layout only

- Pros: groups code by feature
- Cons: boundary enforcement is easy to violate without strict module rules

2. Single-package service layer

- Pros: fast initial coding
- Cons: poor separation, harder testing, high long-term coupling

## Consequences

Positive:

- domain logic remains testable without infrastructure
- failures are easier to localize by layer
- supports strict error taxonomy and transport mapping
- easier to scale with additional adapters

Negative:

- more upfront structure and file movement
- additional discipline needed when adding new packages

## Follow-up actions

- Enforce the boundary rule in code reviews
- Keep ADRs for major architectural changes
- Add integration tests per adapter once persistence and transport are implemented
