package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Godswilly/fingo/internal/database/postgres"
	"github.com/Godswilly/fingo/internal/domain/idempotency"
	"github.com/Godswilly/fingo/internal/errs"
)

func TestIdempotencyStore_SaveCompleted_RejectsEmptyResultRef(t *testing.T) {
	t.Parallel()

	key, err := idempotency.NewKey("unit-key")
	if err != nil {
		t.Fatal(err)
	}

	err = postgres.NewIdempotencyStore(&postgres.DB{}).SaveCompleted(context.Background(), key, "   ")
	if !errors.Is(err, postgres.ErrResultRefRequired) {
		t.Fatalf("expected ErrResultRefRequired, got %v", err)
	}
	if !errs.IsCode(err, errs.CodeInvariantViolation) {
		t.Fatalf("expected invariant violation code, got %v", err)
	}
}
