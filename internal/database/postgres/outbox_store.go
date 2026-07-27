package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Godswilly/fingo/internal/app/ledger"
	dbgen "github.com/Godswilly/fingo/internal/database/db"
	"github.com/Godswilly/fingo/internal/errs"
)

// OutboxStore implements internal/app/ledger.OutboxStore.
type OutboxStore struct {
	db *DB
}

func NewOutboxStore(db *DB) *OutboxStore {
	return &OutboxStore{db: db}
}

// Save persists an outbox event. Callers invoke this within the same
// Transactor.WithinTx closure as the journal write, so it commits or rolls
// back atomically with it.
func (s *OutboxStore) Save(ctx context.Context, event ledger.OutboxEvent) error {
	const op = "postgres.OutboxStore.Save"

	q, err := s.db.txQueries(ctx, op)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return errs.Infrastructure(errs.CodeInternal, op, "failed to encode outbox payload", err)
	}

	if err := q.InsertOutboxEvent(ctx, dbgen.InsertOutboxEventParams{
		Type:        event.Type,
		AggregateID: event.AggregateID,
		Payload:     payload,
		OccurredAt: pgtype.Timestamptz{
			Time:  event.OccurredAt,
			Valid: true,
		},
	}); err != nil {
		return wrapErr(op, err)
	}

	return nil
}
