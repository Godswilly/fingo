# FinGo Roadmap

This roadmap is the project spine: start here, then follow the linked documents
for deeper product, architecture, and operating details.

## Source Documents

- Product scope: [PRD v1](prd/fingo-prd-v1.md)
- System design: [System Design v1](architecture/system-design-v1.md)
- Package rules: [Package Boundaries](architecture/package-boundaries.md)
- Error model: [Error Taxonomy](architecture/error-taxonomy.md)
- Architecture decisions:
  - [ADR-001: Architecture Boundaries](adr/ADR-001-architecture-boundaries.md)
  - [ADR-002: Ledger Invariants and Idempotency Strategy](adr/ADR-002-ledger-invariants-and-idempotency-strategy.md)
- Local workflow: [Dev Environment Runbook](runbooks/dev-environment-runbook.md)

## How To Use These Docs

Use this roadmap as the entry point for planning work.

- Start with the current phase and next feature slice in this file.
- Read the PRD when changing product scope, user stories, success metrics, or
  release-readiness criteria.
- Read the system design when changing service behavior, data flow, reliability
  guarantees, or testing strategy.
- Read package boundaries before adding, moving, or renaming packages.
- Read the error taxonomy before adding new error codes, layers, or transport
  mappings.
- Read ADRs before changing an accepted architectural decision.
- Read the dev runbook when setting up, testing, or debugging the local
  environment.

When a feature changes behavior or architecture, update the specific source
document first. Update this roadmap only when the current phase, status, next
feature slice, or document index changes.

## Current Position

FinGo is currently in late Phase 1 / early Phase 2.

Completed foundation:

- Domain packages for accounts, ledger entries, and idempotency
- Typed layered error model
- Table-driven tests for core domain invariants
- Config loading and validation
- Thin process entrypoints for API, worker, and migrations
- Local quality gates through `make`

The next milestone is to move from domain modeling into application-level
transaction orchestration.

## Execution Phases

### Phase 1: Foundation

Goal: establish correctness-first project structure and local quality gates.

Deliverables:

- Package boundaries under `cmd/` and `internal/`
- Domain entities and value objects
- Typed error taxonomy
- Config validation
- Unit tests, race tests, build checks, and CI-ready `make` targets

Status: mostly complete.

### Phase 2: Transaction Correctness

Goal: prove money-moving use cases are safe before adding external protocols.

Deliverables:

- `internal/app/ledger` application service
- App-layer ports for transaction, account, journal, idempotency, and outbox stores
- `CreateTransfer` use case with context-aware orchestration
- Atomic idempotency reservation for new keys and failed-record retry claims
- Idempotency handling for execute, replay, retry-later, and conflict decisions
- Authoritative replay by loading the journal referenced by completed idempotency records
- Transactional outbox event persistence in the same commit as the journal write
- Unit tests with fakes for failure modes and transaction behavior
- Postgres schema and migrations after the app contract is stable
- Integration tests for row locks, idempotency, and concurrent transfers

Status: next active phase.

### Phase 3: API and Outbox

Goal: expose the proven application service and publish reliable events.

Deliverables:

- API contract for transfer creation and replay semantics
- Transport error mapping for HTTP and/or gRPC
- Transactional outbox persistence
- Worker loop for outbox publishing
- At-least-once delivery behavior with idempotent consumers

Status: planned.

### Phase 4: Production Hardening

Goal: make the system observable, resilient, and operationally safe.

Deliverables:

- Structured logging with request and correlation IDs
- Metrics for throughput, latency, error rate, and outbox lag
- Distributed tracing on write paths
- Retry, backoff, deadlock, and timeout policies
- Runbooks for common failure modes
- Performance benchmarks and profiling baseline

Status: planned.

## Next Feature Slice

Implement the transaction-safe transfer application service.

Scope:

- Add `internal/app/ledger`
- Define `CreateTransferCommand`
- Define app-layer ports:
  - `Transactor`
  - `AccountStore`
  - `JournalStore`
  - `IdempotencyStore`
  - `OutboxStore`
- Implement `TransferService.CreateTransfer(ctx, cmd)`
- Use atomic idempotency reservation and failed-row compare-and-swap semantics
- Save a transfer-created outbox event with the same transaction context
- Reuse existing domain packages for account, ledger, idempotency, and errors
- Add fake-backed tests for valid transfer, replay, conflict, retry-later,
  insufficient funds, failed-idempotency retry behavior, and reservation races

Out of scope:

- Postgres adapter
- gRPC or HTTP handlers
- NATS or JetStream integration
- Docker or deployment changes

## Working Rule

Prefer narrow branches that prove one architectural layer at a time:

1. Domain rules
2. Application orchestration
3. Persistence adapters
4. Transport adapters
5. Messaging adapters
6. Observability and hardening

This keeps correctness visible and prevents infrastructure work from hiding
unfinished business behavior.
