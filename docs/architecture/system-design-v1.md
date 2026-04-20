# FinGo System Design v1

- Status: Draft v1 (execution baseline)
- Date: 2026-04-20
- Scope: correctness-first distributed ledger backend

## 1) Objectives and Constraints

Primary objectives:

- Absolute financial correctness (no money creation/loss)
- High throughput with bounded latency
- Safe operation under retries, partial failures, and restarts

Non-negotiable constraints:

- Monetary values represented as atomic integers (`int64`)
- Ledger writes are ACID transactions
- All money-moving commands are idempotent
- Ledger journal is immutable (append-only semantics)

## 2) Architecture Overview

FinGo uses a layered hexagonal structure:

- `cmd/*` bootstraps processes
- `internal/domain/*` enforces business invariants
- `internal/app/*` orchestrates use-cases
- `internal/transport/*` exposes API protocols
- `internal/persistence/*` handles Postgres I/O
- `internal/messaging/*` handles NATS publish/consume

Dependency direction is inward:

`transport/persistence/messaging -> app -> domain`

## 3) Process and Container View

### `fingo-api`

- Accepts API requests
- Validates commands and auth context
- Calls application services
- Returns mapped transport errors

### `fingo-worker`

- Reads outbox/events
- Publishes events reliably
- Runs background reconciliation/retry loops

### `fingo-migrate`

- Applies schema migrations
- Supports safe startup/deploy workflows

External dependencies:

- Postgres (source of truth)
- NATS JetStream (event delivery)
- Observability backend (logs/metrics/traces)

## 4) Core Domain and Data Model

Core entities:

- `accounts` (identity and status)
- `ledger_journal` (immutable transfer envelope)
- `ledger_postings` (debit/credit rows)
- `idempotency_keys` (dedupe key + outcome reference)
- `outbox_events` (transactional event queue)

Hard invariants:

- For each journal entry: sum(postings) == 0
- Historical postings are never mutated in place
- Balance is derived from postings (snapshots are optimization)
- Duplicate idempotency key must not produce duplicate effects

## 5) Golden Write Path (Create Transfer)

1. Receive command with idempotency key.
2. Validate input and domain preconditions.
3. Start DB transaction.
4. Acquire required locks for affected accounts.
5. Insert journal + postings.
6. Enforce zero-sum invariant before commit.
7. Persist idempotency outcome.
8. Insert outbox event in same transaction.
9. Commit transaction.
10. Worker publishes outbox event with retry/backoff.

This prevents dual-write inconsistencies (DB commit without event or vice versa).

## 6) Consistency and Failure Semantics

Write-side consistency:

- Strong consistency per transaction in Postgres
- At-least-once event publishing via outbox + idempotent consumers

Failure handling:

- Retry transient infra failures with bounded exponential backoff + jitter
- Handle deadlocks with bounded retry policy
- Preserve idempotency across client and worker retries

## 7) API and Error Model

Error model is layered:

- `domain` for invariant violations
- `application` for orchestration/policy failures
- `infrastructure` for DB/network/broker failures

Transport mapping:

- Map typed error codes to deterministic HTTP/gRPC responses
- Never leak internal infra details in client-facing messages

## 8) Security and Audit Baseline

- Authentication and authorization at transport boundary
- Audit record for every money-moving operation
- Secret values never logged
- Principle of least privilege for DB/service accounts

## 9) Observability and SRE Baseline

Required telemetry:

- Structured logs with request/correlation IDs
- Metrics: throughput, error rate, p95 latency, queue/outbox lag
- Traces on critical write/read paths

Operational controls:

- SLI/SLO definitions
- Alert thresholds linked to runbooks
- Periodic failure drills and post-incident notes

## 10) Performance Strategy

Early performance controls:

- Query-count budget for hot endpoints
- Allocation budget for core command paths
- Baseline benchmarks and periodic `pprof` profiling

Optimization principle:

- Optimize only after correctness and observability are in place

## 11) Deployment and Migration Safety

- Use backward-compatible migration strategy (expand/migrate/contract)
- Verify rollback path before high-risk schema changes
- Keep migration execution in dedicated process (`fingo-migrate`)

## 12) Testing Strategy

Minimum testing layers:

- Unit tests for domain invariants and error contracts
- Integration tests for transaction semantics and DB locking behavior
- Race-enabled tests for concurrency-sensitive flows
- Failure-mode tests for retries/idempotency/outbox recovery

## 13) Near-Term Execution Plan

Weeks 1-2:

- Domain invariants + typed errors + table-driven tests
- Config validation and CI hard gates

Weeks 3-4:

- Schema design + migrations + transaction correctness

Weeks 5-8:

- API contracts + idempotency + outbox implementation

Weeks 9+:

- SLOs, resilience drills, profiling, and production hardening

## 14) Open Decisions (Track via ADRs)

- Snapshot strategy for high-volume balance reads
- Exact lock ordering policy for multi-account transactions
- Event schema versioning and compatibility contract
- Reconciliation cadence and escalation thresholds
