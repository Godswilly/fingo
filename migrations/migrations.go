// Package migrations applies plain SQL migration files to a Postgres
// database, tracking applied versions in a schema_migrations table.
package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Godswilly/fingo/internal/errs"
)

//go:embed *.sql
var files embed.FS

const createMigrationsTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    text PRIMARY KEY,
    checksum   text,
    applied_at timestamptz NOT NULL DEFAULT now()
)`

const migrationAdvisoryLockKey int64 = 4919336464568310350

var ErrChecksumMismatch = errors.New("migration checksum mismatch")

type migrationDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Run applies all pending embedded migrations, in filename order, each in
// its own transaction. Already-applied versions are skipped.
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	const op = "migrations.Run"

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return errs.Infrastructure(errs.CodeUnavailable, op, "failed to acquire migration connection", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationAdvisoryLockKey); err != nil {
		return errs.Infrastructure(errs.CodeUnavailable, op, "failed to acquire migration lock", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLockKey)
	}()

	if _, err := conn.Exec(ctx, createMigrationsTableSQL); err != nil {
		return errs.Infrastructure(errs.CodeInternal, op, "failed to ensure schema_migrations table", err)
	}
	if _, err := conn.Exec(ctx, `ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum text`); err != nil {
		return errs.Infrastructure(errs.CodeInternal, op, "failed to ensure schema_migrations checksum column", err)
	}

	entries, err := files.ReadDir(".")
	if err != nil {
		return errs.Infrastructure(errs.CodeInternal, op, "failed to read embedded migrations", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		contents, err := files.ReadFile(name)
		if err != nil {
			return errs.Infrastructure(errs.CodeInternal, op, "failed to read migration file", err)
		}
		checksum := checksumSQL(contents)

		applied, err := checkApplied(ctx, conn, name, checksum)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, conn, name, contents, checksum); err != nil {
			return err
		}
	}

	return nil
}

func checkApplied(ctx context.Context, db migrationDB, version, checksum string) (bool, error) {
	const op = "migrations.checkApplied"

	var existingChecksum *string
	err := db.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE version = $1`, version).Scan(&existingChecksum)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	case err != nil:
		return false, errs.Infrastructure(errs.CodeInternal, op, "failed to check migration state", err)
	}

	if existingChecksum == nil || strings.TrimSpace(*existingChecksum) == "" {
		if _, err := db.Exec(ctx, `UPDATE schema_migrations SET checksum = $2 WHERE version = $1`, version, checksum); err != nil {
			return false, errs.Infrastructure(errs.CodeInternal, op, "failed to backfill migration checksum", err)
		}
		return true, nil
	}

	if *existingChecksum != checksum {
		return false, errs.Infrastructure(
			errs.CodeInvariantViolation,
			op,
			"applied migration checksum does not match embedded migration",
			fmt.Errorf("%w: %s", ErrChecksumMismatch, version),
		)
	}
	return true, nil
}

func applyMigration(ctx context.Context, db migrationDB, version string, contents []byte, checksum string) error {
	const op = "migrations.applyMigration"

	tx, err := db.Begin(ctx)
	if err != nil {
		return errs.Infrastructure(errs.CodeInternal, op, "failed to begin migration transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, string(contents)); err != nil {
		return errs.Infrastructure(errs.CodeInternal, op, "failed to apply migration "+version, err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)`, version, checksum); err != nil {
		return errs.Infrastructure(errs.CodeInternal, op, "failed to record migration "+version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return errs.Infrastructure(errs.CodeInternal, op, "failed to commit migration "+version, err)
	}

	return nil
}

func checksumSQL(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
