package ledger

import (
	"context"
	"time"

	"github.com/Godswilly/fingo/internal/domain/account"
	"github.com/Godswilly/fingo/internal/domain/idempotency"
	domainledger "github.com/Godswilly/fingo/internal/domain/ledger"
)

// Transactor defines the application transaction boundary.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

// AccountStore provides locked account snapshots for transfer validation.
// Implementations must lock the account identity row and return a balance
// snapshot computed consistently from committed postings within the transaction.
type AccountStore interface {
	GetForUpdate(ctx context.Context, id string) (*account.Account, error)
}

// JournalStore persists immutable ledger entries.
type JournalStore interface {
	Get(ctx context.Context, id string) (*domainledger.JournalEntry, error)
	Save(ctx context.Context, entry *domainledger.JournalEntry) error
}

// IdempotencyStore persists idempotency state within the transaction boundary.
type IdempotencyStore interface {
	// Reserve atomically inserts an in_progress row when no row exists.
	// If a row exists, it must be returned under a lock with reserved=false.
	Reserve(ctx context.Context, key idempotency.Key, fingerprint string) (existing *idempotency.Record, reserved bool, err error)

	// MarkInProgress performs a compare-and-swap transition from failed to
	// in_progress for the same key and fingerprint. ok=false means another
	// caller won the retry claim; current must contain the fresh locked row.
	MarkInProgress(ctx context.Context, key idempotency.Key, fingerprint string) (ok bool, current *idempotency.Record, err error)

	SaveCompleted(ctx context.Context, key idempotency.Key, resultRef string) error
}

type OutboxEvent struct {
	OccurredAt  time.Time
	Type        string
	AggregateID string
	Payload     map[string]string
}

// OutboxStore persists an event in the same transaction as the journal write.
type OutboxStore interface {
	Save(ctx context.Context, event OutboxEvent) error
}
