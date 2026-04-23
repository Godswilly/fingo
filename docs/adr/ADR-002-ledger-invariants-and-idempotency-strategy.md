# ADR-002: Ledger Invariants and Idempotency Strategy

- Status: Accepted
- Date: 2026-04-23
- Decision makers: FinGo core maintainer

## Context

FinGo is a correctness-first ledger. The core risk is duplicate or inconsistent money movement caused by retries, partial failures, or malformed posting sets.

Without explicit domain invariants and idempotency behavior:

- debit/credit sets can become inconsistent
- mixed-currency postings can be accidentally balanced numerically
- duplicate requests can create duplicate financial effects
- recovery paths become ambiguous under failure/retry conditions

Week 2 introduced immutable ledger posting/journal models and domain idempotency guard logic. We need an explicit architecture decision that defines the invariant contract and failure semantics.

## Decision

Adopt the following domain-level correctness contract.

Ledger posting invariants (enforced in domain):

- Posting amount is positive (`int64` minor units)
- Posting side is explicit (`debit` or `credit`)
- Journal entry has at least 2 postings
- Sum(debits) equals sum(credits)
- Posting totals must not overflow `int64`
- All non-empty posting currencies in a journal entry must match (single-currency entry)
- Journal and postings are append-only (no in-place historical mutation)

Idempotency strategy (enforced in domain):

- Every money-moving command provides an idempotency key
- Command fingerprint is deterministic from canonical command fields
- Decision matrix for an incoming request:
  - no existing record -> `execute`
  - same fingerprint + `completed` -> `replay`
  - same fingerprint + `in_progress` -> `retry_later`
  - same fingerprint + `failed` -> `execute`
  - different fingerprint for same key -> `reject_conflict`
- Empty/invalid keys or fingerprints are rejected as invalid argument
- Unknown persisted idempotency status is treated as invariant violation

Error semantics:

- rule/input violations -> `invalid_argument`
- idempotency key reuse with different fingerprint -> `conflict`
- broken internal idempotency state or ledger invariant breach -> `invariant_violation`

## Alternatives considered

1. Database-only enforcement (minimal domain rules)

- Pros: fewer domain checks, leverage DB constraints
- Cons: correctness is delayed to persistence layer, weaker local testability, less explicit failure semantics in application flow

2. Signed amount model (no explicit debit/credit side)

- Pros: simpler arithmetic
- Cons: easier to mix sign conventions, less readable accounting intent, higher risk of incorrect posting construction

3. Idempotency key only (no fingerprint comparison)

- Pros: minimal implementation complexity
- Cons: same key could be reused for a different command payload without deterministic conflict detection

## Failure-mode analysis

1. Client timeout then retry with same command

- Risk: duplicate transfer
- Control: same key + same fingerprint resolves to replay/execute-safe path

2. Same idempotency key reused with modified amount/reference

- Risk: key collision causes unintended acceptance
- Control: fingerprint mismatch returns conflict (`reject_conflict`)

3. Concurrent duplicate requests while first request is still processing

- Risk: double execution race
- Control: `in_progress` status maps to `retry_later`

4. Unbalanced posting set

- Risk: money creation/loss
- Control: domain rejects on `sum(debits) != sum(credits)`

5. Numerically balanced but mixed-currency posting set

- Risk: semantic financial corruption
- Control: single-currency invariant rejects mixed currencies

6. Very large posting totals

- Risk: integer overflow corrupts totals
- Control: overflow checks on debit/credit accumulation reject entry

7. Corrupted persisted idempotency status

- Risk: undefined behavior in retries
- Control: unknown status treated as invariant violation and rejected

## Consequences

Positive:

- correctness rules are explicit and testable at domain layer
- idempotency behavior is deterministic and auditable
- retry behavior is safe under common distributed failure scenarios
- error handling is consistent with the established taxonomy

Negative:

- more upfront domain modeling and tests
- stricter validation can reject previously accepted malformed inputs
- persistence layer must preserve state transitions expected by the decision matrix

## Follow-up actions

- Implement persistence adapter for idempotency records with transactional guarantees
- Add integration tests for DB transaction + idempotency behavior under concurrency
- Document API-level idempotency contract in transport docs (request headers/fields and replay semantics)
- Track status transition rules in runbooks and incident response guides
