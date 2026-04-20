# Product Requirements Document (PRD) — FinGo

## 1. Product Summary

- Product name: FinGo
- Type: High-performance, distributed financial ledger backend
- Purpose: Act as the source of truth for money movement with strict correctness, reliability, and auditability.

## 2. Problem Statement

Financial systems fail when ledger correctness, idempotency, and reliability are treated as secondary concerns. FinGo solves this by enforcing immutable, double-entry, transaction-safe ledger operations with operational visibility and failure resilience.

## 3. Goals

1. Guarantee financial correctness (no money loss/creation).
2. Support safe high-throughput transaction processing.
3. Provide reliable event propagation to downstream systems.
4. Offer production-ready observability and operational controls.

## 4. Non-Goals (v1)

1. End-user UI/dashboard.
2. Multi-region active-active replication.
3. Full compliance feature set (KYC/AML workflows).
4. Complex settlement/netting engine.

## 5. Target Users

1. Backend/platform engineers integrating financial workflows.
2. Internal services requiring a trusted ledger source of truth.
3. SRE/operations teams responsible for reliability and incident response.

## 6. Core User Stories

1. As a service, I can submit a transfer once and avoid duplicate effects via idempotency.
2. As an engineer, I can trust that every transfer is ACID-safe and double-entry balanced.
3. As an operator, I can detect failures through logs/metrics and follow runbooks.
4. As a downstream consumer, I receive reliable ledger events via outbox + messaging.

## 7. Functional Requirements

1. Account and ledger journal management.
2. Double-entry postings with enforced zero-sum invariants.
3. Idempotency key handling for write operations.
4. Transactional outbox for event publication.
5. Worker for event delivery and retry handling.
6. API surface (REST/gRPC) with deterministic error mapping.
7. Migration runner for schema lifecycle control.
8. Audit logging for money-moving operations.

## 8. Non-Functional Requirements

1. Monetary precision: `int64` atomic units only.
2. Consistency: ACID transactions for all writes.
3. Reliability: retries with backoff + jitter; deadlock handling.
4. Performance: defined p95 latency and allocation/query budgets.
5. Security: authn/authz, secret hygiene, least privilege.
6. Observability: structured logs, metrics, traces, alertability.
7. Operability: runbooks, failure drills, rollback paths.

## 9. Success Metrics

1. Zero invariant violations in production paths.
2. Duplicate write prevention rate: 100% on repeated idempotency keys.
3. CI quality gate pass rate on protected branches.
4. SLO attainment (latency/error targets) after baseline period.
5. Successful backup/restore drills meeting defined RPO/RTO.

## 10. Scope by Phases (16-week plan alignment)

1. Phase 1: Foundation (structure, config, errors, CI, tests).
2. Phase 2: Domain + persistence correctness (schema, transactions, idempotency).
3. Phase 3: API + outbox + observability baseline.
4. Phase 4: Performance, resilience, release hardening.

## 11. Risks

1. Invariant drift between domain and DB constraints.
2. Idempotency bypass under concurrency.
3. Outbox/event divergence.
4. Migration downtime risk.
5. Restore path reliability gaps.

## 12. Release Readiness Criteria (v1)

1. All hard quality gates passing (`fmt/lint/test/race/build`).
2. Critical domain + transaction + idempotency tests passing.
3. Outbox reliability tests passing.
4. SLO/alert/runbook baseline present.
5. DR drill evidence documented.

## 13. Open Questions

1. Snapshot strategy and refresh model for balance reads.
2. Event schema versioning policy.
3. Consumer idempotency contract shape.
4. Lock ordering policy for multi-account transfers.
