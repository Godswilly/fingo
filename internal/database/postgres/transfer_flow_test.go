//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Godswilly/fingo/internal/app/ledger"
	"github.com/Godswilly/fingo/internal/database/postgres"
	"github.com/Godswilly/fingo/internal/domain/account"
	"github.com/Godswilly/fingo/internal/domain/idempotency"
	"github.com/Godswilly/fingo/internal/errs"
)

func newTransferService(t *testing.T) *ledger.TransferService {
	t.Helper()

	svc, err := ledger.NewTransferService(ledger.TransferServiceDeps{
		Transactor:    postgres.NewTransactor(testDB),
		Accounts:      postgres.NewAccountStore(testDB),
		Journals:      postgres.NewJournalStore(testDB),
		Idempotencies: postgres.NewIdempotencyStore(testDB),
		Outbox:        postgres.NewOutboxStore(testDB),
	})
	if err != nil {
		t.Fatalf("new transfer service: %v", err)
	}
	return svc
}

func TestTransferFlow_ValidTransfer(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "alice", "Alice")
	insertAccount(t, "bob", "Bob")
	seedBalance(t, "alice", 10000, "USD")

	svc := newTransferService(t)
	ctx := context.Background()
	cmd := ledger.CreateTransferCommand{
		ID:             "xfer-1",
		IdempotencyKey: "idem-1",
		Reference:      "ref-1",
		Description:    "integration test transfer",
		FromAccountID:  "alice",
		ToAccountID:    "bob",
		Currency:       "USD",
		Amount:         2500,
		CreatedAt:      time.Now().UTC(),
	}

	result, err := svc.CreateTransfer(ctx, cmd)
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	if result.JournalID != "xfer-1" || result.Replayed {
		t.Fatalf("unexpected result: %+v", result)
	}

	accounts := postgres.NewAccountStore(testDB)
	var alice, bob *account.Account
	withinTx(t, func(txCtx context.Context) error {
		var err error
		alice, err = accounts.GetForUpdate(txCtx, "alice")
		if err != nil {
			return err
		}
		bob, err = accounts.GetForUpdate(txCtx, "bob")
		return err
	})
	if alice.Balance != 7500 {
		t.Fatalf("expected alice balance 7500, got %d", alice.Balance)
	}
	if bob.Balance != 2500 {
		t.Fatalf("expected bob balance 2500, got %d", bob.Balance)
	}
}

func TestTransferFlow_Replay(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "alice", "Alice")
	insertAccount(t, "bob", "Bob")
	seedBalance(t, "alice", 10000, "USD")

	svc := newTransferService(t)
	ctx := context.Background()
	cmd := ledger.CreateTransferCommand{
		ID:             "xfer-2",
		IdempotencyKey: "idem-2",
		Reference:      "ref-2",
		FromAccountID:  "alice",
		ToAccountID:    "bob",
		Currency:       "USD",
		Amount:         1000,
		CreatedAt:      time.Now().UTC(),
	}

	first, err := svc.CreateTransfer(ctx, cmd)
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}

	second, err := svc.CreateTransfer(ctx, cmd)
	if err != nil {
		t.Fatalf("replayed transfer: %v", err)
	}
	if !second.Replayed {
		t.Fatal("expected the second call to be a replay")
	}
	if second.JournalID != first.JournalID {
		t.Fatalf("expected same journal id, got %s vs %s", second.JournalID, first.JournalID)
	}

	accounts := postgres.NewAccountStore(testDB)
	var bob *account.Account
	withinTx(t, func(txCtx context.Context) error {
		var err error
		bob, err = accounts.GetForUpdate(txCtx, "bob")
		return err
	})
	if bob.Balance != 1000 {
		t.Fatalf("expected bob balance 1000 (no duplicate effect from replay), got %d", bob.Balance)
	}
}

func TestTransferFlow_ConflictOnFingerprintMismatch(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "alice", "Alice")
	insertAccount(t, "bob", "Bob")
	seedBalance(t, "alice", 10000, "USD")

	svc := newTransferService(t)
	ctx := context.Background()
	base := ledger.CreateTransferCommand{
		ID:             "xfer-3",
		IdempotencyKey: "idem-3",
		Reference:      "ref-3",
		FromAccountID:  "alice",
		ToAccountID:    "bob",
		Currency:       "USD",
		Amount:         1000,
		CreatedAt:      time.Now().UTC(),
	}
	if _, err := svc.CreateTransfer(ctx, base); err != nil {
		t.Fatalf("first transfer: %v", err)
	}

	conflicting := base
	conflicting.ID = "xfer-3b"
	conflicting.Amount = 2000 // same idempotency key, different fingerprint

	_, err := svc.CreateTransfer(ctx, conflicting)
	if err == nil {
		t.Fatal("expected a conflict error for a reused key with a different command")
	}
	if !errs.IsCode(err, errs.CodeConflict) {
		t.Fatalf("expected conflict code, got %v", err)
	}
}

func TestTransferFlow_RetryLaterWhileInProgress(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "alice", "Alice")
	insertAccount(t, "bob", "Bob")
	seedBalance(t, "alice", 10000, "USD")

	ctx := context.Background()
	cmd := ledger.CreateTransferCommand{
		ID:             "xfer-4",
		IdempotencyKey: "idem-4",
		Reference:      "ref-4",
		FromAccountID:  "alice",
		ToAccountID:    "bob",
		Currency:       "USD",
		Amount:         1000,
		CreatedAt:      time.Now().UTC(),
	}

	fingerprint := transferFingerprint(t, cmd.FromAccountID, cmd.ToAccountID, cmd.Currency, cmd.Reference, cmd.Amount)
	key, err := idempotency.NewKey(cmd.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}

	idem := postgres.NewIdempotencyStore(testDB)
	withinTx(t, func(txCtx context.Context) error {
		_, reserved, err := idem.Reserve(txCtx, key, fingerprint)
		if err != nil || !reserved {
			t.Fatalf("pre-reserve: reserved=%v err=%v", reserved, err)
		}
		return nil
	})

	svc := newTransferService(t)
	_, err = svc.CreateTransfer(ctx, cmd)
	if err == nil {
		t.Fatal("expected a retry-later error while the key is in_progress")
	}
	if !errs.IsCode(err, errs.CodeConflict) {
		t.Fatalf("expected conflict code for retry_later, got %v", err)
	}
}

func TestTransferFlow_FailedRetryClaimExecutesExactlyOnce(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "alice", "Alice")
	insertAccount(t, "bob", "Bob")
	seedBalance(t, "alice", 10000, "USD")

	ctx := context.Background()
	cmd := ledger.CreateTransferCommand{
		ID:             "xfer-5",
		IdempotencyKey: "idem-5",
		Reference:      "ref-5",
		FromAccountID:  "alice",
		ToAccountID:    "bob",
		Currency:       "USD",
		Amount:         1000,
		CreatedAt:      time.Now().UTC(),
	}

	fingerprint := transferFingerprint(t, cmd.FromAccountID, cmd.ToAccountID, cmd.Currency, cmd.Reference, cmd.Amount)
	key, err := idempotency.NewKey(cmd.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}

	idem := postgres.NewIdempotencyStore(testDB)
	withinTx(t, func(txCtx context.Context) error {
		_, reserved, err := idem.Reserve(txCtx, key, fingerprint)
		if err != nil || !reserved {
			t.Fatalf("pre-reserve: reserved=%v err=%v", reserved, err)
		}
		return nil
	})
	if _, err := testDB.Pool.Exec(ctx, `UPDATE idempotency_keys SET status = 'failed' WHERE key = $1`, key.String()); err != nil {
		t.Fatalf("force failed status: %v", err)
	}

	const n = 5
	results := make([]ledger.CreateTransferResult, n)
	callErrs := make([]error, n)
	var wg sync.WaitGroup
	svc := newTransferService(t)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], callErrs[i] = svc.CreateTransfer(ctx, cmd)
		}(i)
	}
	wg.Wait()

	executed := 0
	for i := 0; i < n; i++ {
		if callErrs[i] == nil && !results[i].Replayed {
			executed++
		}
	}
	if executed != 1 {
		t.Fatalf("expected exactly one non-replay execution among concurrent retries, got %d", executed)
	}

	accounts := postgres.NewAccountStore(testDB)
	var bob *account.Account
	withinTx(t, func(txCtx context.Context) error {
		var err error
		bob, err = accounts.GetForUpdate(txCtx, "bob")
		return err
	})
	if bob.Balance != 1000 {
		t.Fatalf("expected bob balance 1000 reflecting a single execution, got %d", bob.Balance)
	}
}

func TestTransferFlow_OutboxAtomicity(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "alice", "Alice")
	insertAccount(t, "bob", "Bob")
	seedBalance(t, "alice", 10000, "USD")

	svc := newTransferService(t)
	ctx := context.Background()
	cmd := ledger.CreateTransferCommand{
		ID:             "xfer-6",
		IdempotencyKey: "idem-6",
		Reference:      "ref-6",
		FromAccountID:  "alice",
		ToAccountID:    "bob",
		Currency:       "USD",
		Amount:         750,
		CreatedAt:      time.Now().UTC(),
	}

	result, err := svc.CreateTransfer(ctx, cmd)
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}

	var journalExists, outboxExists bool
	if scanErr := testDB.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ledger_journal WHERE id = $1)`, result.JournalID,
	).Scan(&journalExists); scanErr != nil {
		t.Fatal(scanErr)
	}
	if scanErr := testDB.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM outbox_events WHERE aggregate_id = $1 AND type = 'ledger.transfer.created')`, result.JournalID,
	).Scan(&outboxExists); scanErr != nil {
		t.Fatal(scanErr)
	}

	if !journalExists || !outboxExists {
		t.Fatalf("expected both journal and outbox rows to exist together: journal=%v outbox=%v", journalExists, outboxExists)
	}
}
