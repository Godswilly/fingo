package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	dbgen "github.com/Godswilly/fingo/internal/database/db"
	"github.com/Godswilly/fingo/internal/errs"
)

type ctxKeyTx struct{}

var ErrTransactionRequired = errors.New("postgres adapter method requires Transactor.WithinTx")

// Transactor implements internal/app/ledger.Transactor against Postgres,
// threading a pgx.Tx through the context so all ports opened within
// WithinTx share the same database transaction.
type Transactor struct {
	db *DB
}

func NewTransactor(db *DB) *Transactor {
	return &Transactor{db: db}
}

func (t *Transactor) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	const op = "postgres.WithinTx"

	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := t.db.Pool.Begin(ctx)
	if err != nil {
		return errs.Infrastructure(errs.CodeUnavailable, op, "failed to begin transaction", err)
	}

	txCtx := context.WithValue(ctx, ctxKeyTx{}, tx)

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return errs.Infrastructure(errs.CodeUnavailable, op, "failed to commit transaction", err)
	}

	return nil
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(ctxKeyTx{}).(pgx.Tx)
	return tx, ok
}

func (db *DB) queries() *dbgen.Queries {
	return dbgen.New(db.Pool)
}

func (db *DB) queriesForContext(ctx context.Context) *dbgen.Queries {
	if tx, ok := txFromContext(ctx); ok {
		return dbgen.New(tx)
	}
	return db.queries()
}

func (db *DB) txQueries(ctx context.Context, op string) (*dbgen.Queries, error) {
	tx, ok := txFromContext(ctx)
	if !ok {
		return nil, errs.Infrastructure(
			errs.CodeInternal,
			op,
			"postgres adapter method requires transaction context",
			ErrTransactionRequired,
		)
	}
	return dbgen.New(tx), nil
}
