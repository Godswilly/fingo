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

func TestStoresRequireTransactionForLockingAndMutatingMethods(t *testing.T) {
	t.Parallel()

	db := &postgres.DB{}
	ctx := context.Background()

	key, err := idempotency.NewKey("unit-key")
	if err != nil {
		t.Fatal(err)
	}

	debit, err := domainledger.NewPosting("from", 100, domainledger.SideDebit, "USD")
	if err != nil {
		t.Fatal(err)
	}
	credit, err := domainledger.NewPosting("to", 100, domainledger.SideCredit, "USD")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := domainledger.NewJournalEntry(domainledger.CreateJournalEntryInput{
		ID:        "journal-unit",
		Reference: "ref-unit",
		CreatedAt: time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC),
		Postings:  []domainledger.Posting{debit, credit},
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		run  func() error
	}{
		{
			name: "account lock",
			run: func() error {
				_, err := postgres.NewAccountStore(db).GetForUpdate(ctx, "acct")
				return err
			},
		},
		{
			name: "journal save",
			run: func() error {
				return postgres.NewJournalStore(db).Save(ctx, entry)
			},
		},
		{
			name: "idempotency reserve",
			run: func() error {
				_, _, err := postgres.NewIdempotencyStore(db).Reserve(ctx, key, "fp")
				return err
			},
		},
		{
			name: "idempotency mark in progress",
			run: func() error {
				_, _, err := postgres.NewIdempotencyStore(db).MarkInProgress(ctx, key, "fp")
				return err
			},
		},
		{
			name: "idempotency save completed",
			run: func() error {
				return postgres.NewIdempotencyStore(db).SaveCompleted(ctx, key, "journal-unit")
			},
		},
		{
			name: "outbox save",
			run: func() error {
				return postgres.NewOutboxStore(db).Save(ctx, ledger.OutboxEvent{
					Type:        "ledger.transfer.created",
					AggregateID: "journal-unit",
					OccurredAt:  time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC),
					Payload:     map[string]string{"journal_id": "journal-unit"},
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.run(); !errors.Is(err, postgres.ErrTransactionRequired) {
				t.Fatalf("expected ErrTransactionRequired, got %v", err)
			}
		})
	}
}
