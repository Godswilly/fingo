package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Godswilly/fingo/internal/errs"
)

const (
	pgErrUniqueViolation     = "23505"
	pgErrForeignKeyViolation = "23503"
	pgErrCheckViolation      = "23514"
)

// wrapErr translates a raw pgx/Postgres error into a typed, safe
// infrastructure error. It never surfaces raw SQL or connection details in
// the returned Message, per docs/architecture/error-taxonomy.md; the
// original error remains available via errors.Unwrap for internal logging.
func wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return errs.Infrastructure(errs.CodeNotFound, op, "resource not found", err)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgErrUniqueViolation:
			return errs.Infrastructure(errs.CodeConflict, op, "constraint conflict", err)
		case pgErrForeignKeyViolation, pgErrCheckViolation:
			return errs.Infrastructure(errs.CodeInvariantViolation, op, "constraint violation", err)
		}
	}

	return errs.Infrastructure(errs.CodeInternal, op, "database operation failed", err)
}
