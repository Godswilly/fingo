// Package postgres implements the app-layer ledger ports
// (internal/app/ledger.Transactor, AccountStore, JournalStore,
// IdempotencyStore, OutboxStore) against a real Postgres database.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Godswilly/fingo/internal/errs"
)

// DB wraps a pgx connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// Open parses dsn, creates a connection pool, and verifies connectivity.
func Open(ctx context.Context, dsn string) (*DB, error) {
	const op = "postgres.Open"

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errs.Infrastructure(errs.CodeInvalidArgument, op, "invalid database connection string", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errs.Infrastructure(errs.CodeUnavailable, op, "failed to create database pool", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errs.Infrastructure(errs.CodeUnavailable, op, "failed to connect to database", err)
	}

	return &DB{Pool: pool}, nil
}

// Close releases all pooled connections.
func (db *DB) Close() {
	db.Pool.Close()
}
