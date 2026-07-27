//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Godswilly/fingo/internal/database/postgres"
	domainledger "github.com/Godswilly/fingo/internal/domain/ledger"
)

func TestJournalStore_SaveAndGet(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "from-acct", "alice")
	insertAccount(t, "to-acct", "bob")

	debit, err := domainledger.NewPosting("from-acct", 1000, domainledger.SideDebit, "USD")
	if err != nil {
		t.Fatalf("new debit posting: %v", err)
	}
	credit, err := domainledger.NewPosting("to-acct", 1000, domainledger.SideCredit, "USD")
	if err != nil {
		t.Fatalf("new credit posting: %v", err)
	}

	entry, err := domainledger.NewJournalEntry(domainledger.CreateJournalEntryInput{
		ID:          "journal-1",
		Reference:   "ref-1",
		Description: "test transfer",
		CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
		Postings:    []domainledger.Posting{debit, credit},
		Metadata:    map[string]string{"note": "integration"},
	})
	if err != nil {
		t.Fatalf("new journal entry: %v", err)
	}

	ctx := context.Background()
	store := postgres.NewJournalStore(testDB)
	withinTx(t, func(txCtx context.Context) error {
		return store.Save(txCtx, entry)
	})

	got, err := store.Get(ctx, "journal-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.ID() != entry.ID() || got.Reference() != entry.Reference() || got.Description() != entry.Description() {
		t.Fatalf("identity mismatch: got %+v", got)
	}
	if !got.CreatedAt().Equal(entry.CreatedAt()) {
		t.Fatalf("created_at mismatch: got %v want %v", got.CreatedAt(), entry.CreatedAt())
	}
	if got.Metadata()["note"] != "integration" {
		t.Fatalf("expected metadata to round-trip, got %+v", got.Metadata())
	}

	postings := got.Postings()
	if len(postings) != 2 {
		t.Fatalf("expected 2 postings, got %d", len(postings))
	}
	if postings[0].AccountID() != "from-acct" || postings[0].Side() != domainledger.SideDebit {
		t.Fatalf("expected first posting to be the debit leg, got %+v", postings[0])
	}
	if postings[1].AccountID() != "to-acct" || postings[1].Side() != domainledger.SideCredit {
		t.Fatalf("expected second posting to be the credit leg, got %+v", postings[1])
	}
}

func TestJournalStore_Get_NotFound(t *testing.T) {
	truncateAll(t)

	store := postgres.NewJournalStore(testDB)
	got, err := store.Get(context.Background(), "missing-journal")
	if err != nil {
		t.Fatalf("expected missing journal to return nil result without error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil journal for missing row, got %+v", got)
	}
}
