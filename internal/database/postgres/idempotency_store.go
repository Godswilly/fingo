package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	dbgen "github.com/Godswilly/fingo/internal/database/db"
	"github.com/Godswilly/fingo/internal/domain/idempotency"
	"github.com/Godswilly/fingo/internal/errs"
)

// IdempotencyStore implements internal/app/ledger.IdempotencyStore.
type IdempotencyStore struct {
	db *DB
}

var ErrResultRefRequired = errors.New("completed idempotency result reference is required")

func NewIdempotencyStore(db *DB) *IdempotencyStore {
	return &IdempotencyStore{db: db}
}

// Reserve atomically inserts an in_progress row when no row exists for key.
// If a row already exists, it is returned locked (FOR UPDATE) with
// reserved=false, per docs/adr/ADR-002.
func (s *IdempotencyStore) Reserve(ctx context.Context, key idempotency.Key, fingerprint string) (*idempotency.Record, bool, error) {
	const op = "postgres.IdempotencyStore.Reserve"

	q, err := s.db.txQueries(ctx, op)
	if err != nil {
		return nil, false, err
	}

	_, err = q.ReserveIdempotency(ctx, dbgen.ReserveIdempotencyParams{
		Key:         key.String(),
		Fingerprint: fingerprint,
		Status:      string(idempotency.StatusInProgress),
	})
	switch {
	case err == nil:
		return nil, true, nil
	case errors.Is(err, pgx.ErrNoRows):
		existing, lockErr := s.lockByKey(ctx, key)
		if lockErr != nil {
			return nil, false, lockErr
		}
		return existing, false, nil
	default:
		return nil, false, wrapErr(op, err)
	}
}

// MarkInProgress performs a compare-and-swap transition from failed to
// in_progress for the same key and fingerprint. ok=false means another
// caller won the retry claim; current contains the fresh locked row.
func (s *IdempotencyStore) MarkInProgress(ctx context.Context, key idempotency.Key, fingerprint string) (bool, *idempotency.Record, error) {
	const op = "postgres.IdempotencyStore.MarkInProgress"

	q, err := s.db.txQueries(ctx, op)
	if err != nil {
		return false, nil, err
	}

	row, err := q.MarkIdempotencyInProgress(ctx, dbgen.MarkIdempotencyInProgressParams{
		Key:         key.String(),
		Fingerprint: fingerprint,
		Status:      string(idempotency.StatusInProgress),
		Status_2:    string(idempotency.StatusFailed),
	})
	switch {
	case err == nil:
		return true, toIdempotencyRecord(row), nil
	case errors.Is(err, pgx.ErrNoRows):
		current, lockErr := s.lockByKey(ctx, key)
		if lockErr != nil {
			return false, nil, lockErr
		}
		return false, current, nil
	default:
		return false, nil, wrapErr(op, err)
	}
}

// SaveCompleted marks a key as completed with its authoritative result
// reference (the created journal ID).
func (s *IdempotencyStore) SaveCompleted(ctx context.Context, key idempotency.Key, resultRef string) error {
	const op = "postgres.IdempotencyStore.SaveCompleted"

	trimmedResultRef := strings.TrimSpace(resultRef)
	if trimmedResultRef == "" {
		return errs.Infrastructure(
			errs.CodeInvariantViolation,
			op,
			"completed idempotency result reference is required",
			fmt.Errorf("%w: %s", ErrResultRefRequired, key.String()),
		)
	}

	q, err := s.db.txQueries(ctx, op)
	if err != nil {
		return err
	}

	affected, err := q.SaveIdempotencyCompleted(ctx, dbgen.SaveIdempotencyCompletedParams{
		Key:       key.String(),
		Status:    string(idempotency.StatusCompleted),
		ResultRef: stringPtr(trimmedResultRef),
	})
	if err != nil {
		return wrapErr(op, err)
	}
	if affected == 0 {
		return errs.Infrastructure(errs.CodeInvariantViolation, op, "idempotency key not found for completion", nil)
	}

	return nil
}

func (s *IdempotencyStore) lockByKey(ctx context.Context, key idempotency.Key) (*idempotency.Record, error) {
	const op = "postgres.IdempotencyStore.lockByKey"

	q, err := s.db.txQueries(ctx, op)
	if err != nil {
		return nil, err
	}

	row, err := q.LockIdempotencyByKey(ctx, key.String())
	if err != nil {
		return nil, wrapErr(op, err)
	}
	return toIdempotencyRecord(row), nil
}

func toIdempotencyRecord(row dbgen.IdempotencyKey) *idempotency.Record {
	rec := &idempotency.Record{
		Key:         idempotency.Key(row.Key),
		Fingerprint: row.Fingerprint,
		Status:      idempotency.Status(row.Status),
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
	if row.ResultRef != nil {
		rec.ResultRef = *row.ResultRef
	}
	return rec
}

func stringPtr(value string) *string {
	return &value
}
