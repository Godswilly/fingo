//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Godswilly/fingo/internal/app/ledger"
	"github.com/Godswilly/fingo/internal/database/postgres"
)

func TestOutboxStore_Save(t *testing.T) {
	truncateAll(t)

	store := postgres.NewOutboxStore(testDB)
	event := ledger.OutboxEvent{
		Type:        "ledger.transfer.created",
		AggregateID: "journal-1",
		OccurredAt:  time.Now().UTC().Truncate(time.Microsecond),
		Payload:     map[string]string{"journal_id": "journal-1", "amount": "100"},
	}

	withinTx(t, func(ctx context.Context) error {
		return store.Save(ctx, event)
	})

	var count int
	err := testDB.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND type = $2`,
		event.AggregateID, event.Type,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 outbox row, got %d", count)
	}
}
