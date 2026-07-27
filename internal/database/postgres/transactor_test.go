//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Godswilly/fingo/internal/app/ledger"
	"github.com/Godswilly/fingo/internal/database/postgres"
	"github.com/Godswilly/fingo/internal/domain/idempotency"
	domainledger "github.com/Godswilly/fingo/internal/domain/ledger"
)

func TestAccountStore_GetForUpdate_BlocksConcurrentLock(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "locked-acct", "carol")

	tx := postgres.NewTransactor(testDB)
	store := postgres.NewAccountStore(testDB)

	firstLocked := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan time.Time, 1)
	secondDone := make(chan time.Time, 1)

	go func() {
		_ = tx.WithinTx(context.Background(), func(ctx context.Context) error {
			if _, err := store.GetForUpdate(ctx, "locked-acct"); err != nil {
				t.Errorf("first GetForUpdate: %v", err)
				return err
			}
			close(firstLocked)
			<-releaseFirst
			return nil
		})
		firstDone <- time.Now()
	}()

	<-firstLocked

	go func() {
		_ = tx.WithinTx(context.Background(), func(ctx context.Context) error {
			if _, err := store.GetForUpdate(ctx, "locked-acct"); err != nil {
				t.Errorf("second GetForUpdate: %v", err)
				return err
			}
			return nil
		})
		secondDone <- time.Now()
	}()

	// The second lock attempt must still be blocked while the first
	// transaction holds the row lock.
	time.Sleep(200 * time.Millisecond)
	select {
	case <-secondDone:
		t.Fatal("expected second GetForUpdate to block while first transaction holds the row lock")
	default:
	}

	close(releaseFirst)
	firstFinishedAt := <-firstDone
	secondFinishedAt := <-secondDone

	if !secondFinishedAt.After(firstFinishedAt) {
		t.Fatalf("expected second lock to be acquired after the first released it: first=%v second=%v",
			firstFinishedAt, secondFinishedAt)
	}
}

func TestTransactor_RollbackDiscardsAllWrites(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "alice", "Alice")
	insertAccount(t, "bob", "Bob")
	seedBalance(t, "alice", 10000, "USD")

	tx := postgres.NewTransactor(testDB)
	journals := postgres.NewJournalStore(testDB)
	idem := postgres.NewIdempotencyStore(testDB)
	outbox := postgres.NewOutboxStore(testDB)
	ctx := context.Background()

	key, err := idempotency.NewKey("rollback-key")
	if err != nil {
		t.Fatal(err)
	}
	sentinelErr := errors.New("forced failure after writes")

	err = tx.WithinTx(ctx, func(txCtx context.Context) error {
		if _, _, err := idem.Reserve(txCtx, key, "fp-rollback"); err != nil {
			return err
		}

		debit, err := domainledger.NewPosting("alice", 500, domainledger.SideDebit, "USD")
		if err != nil {
			return err
		}
		credit, err := domainledger.NewPosting("bob", 500, domainledger.SideCredit, "USD")
		if err != nil {
			return err
		}
		entry, err := domainledger.NewJournalEntry(domainledger.CreateJournalEntryInput{
			ID:        "rollback-journal",
			Reference: "rollback-ref",
			CreatedAt: time.Now().UTC(),
			Postings:  []domainledger.Posting{debit, credit},
		})
		if err != nil {
			return err
		}
		if err := journals.Save(txCtx, entry); err != nil {
			return err
		}
		if err := outbox.Save(txCtx, ledger.OutboxEvent{
			Type:        "ledger.transfer.created",
			AggregateID: entry.ID(),
			OccurredAt:  entry.CreatedAt(),
			Payload:     map[string]string{"journal_id": entry.ID()},
		}); err != nil {
			return err
		}

		return sentinelErr
	})

	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinel error to propagate, got %v", err)
	}

	var journalCount, postingCount, idemCount, outboxCount int
	if scanErr := testDB.Pool.QueryRow(ctx, `SELECT count(*) FROM ledger_journal WHERE id = 'rollback-journal'`).Scan(&journalCount); scanErr != nil {
		t.Fatal(scanErr)
	}
	if scanErr := testDB.Pool.QueryRow(ctx, `SELECT count(*) FROM ledger_postings WHERE journal_id = 'rollback-journal'`).Scan(&postingCount); scanErr != nil {
		t.Fatal(scanErr)
	}
	if scanErr := testDB.Pool.QueryRow(ctx, `SELECT count(*) FROM idempotency_keys WHERE key = $1`, key.String()).Scan(&idemCount); scanErr != nil {
		t.Fatal(scanErr)
	}
	if scanErr := testDB.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = 'rollback-journal'`).Scan(&outboxCount); scanErr != nil {
		t.Fatal(scanErr)
	}

	if journalCount != 0 || postingCount != 0 || idemCount != 0 || outboxCount != 0 {
		t.Fatalf("expected full rollback, got journal=%d postings=%d idempotency=%d outbox=%d",
			journalCount, postingCount, idemCount, outboxCount)
	}
}
