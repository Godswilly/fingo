//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/Godswilly/fingo/internal/database/postgres"
	"github.com/Godswilly/fingo/internal/domain/account"
	"github.com/Godswilly/fingo/internal/errs"
)

func TestAccountStore_GetForUpdate_NotFound(t *testing.T) {
	truncateAll(t)

	store := postgres.NewAccountStore(testDB)
	withinTx(t, func(ctx context.Context) error {
		_, err := store.GetForUpdate(ctx, "missing-account")
		if err == nil {
			t.Fatal("expected error for missing account")
		}
		if !errs.IsCode(err, errs.CodeNotFound) {
			t.Fatalf("expected not_found code, got %v", err)
		}
		return nil
	})
}

func TestAccountStore_GetForUpdate_BalanceFromPostings(t *testing.T) {
	truncateAll(t)
	insertAccount(t, "acct-1", "alice")
	seedBalance(t, "acct-1", 5000, "USD")

	store := postgres.NewAccountStore(testDB)
	var acc *account.Account
	withinTx(t, func(ctx context.Context) error {
		var err error
		acc, err = store.GetForUpdate(ctx, "acct-1")
		return err
	})
	if acc.ID != "acct-1" || acc.Owner != "alice" {
		t.Fatalf("unexpected identity: %+v", acc)
	}
	if !acc.IsActive {
		t.Fatal("expected account to be active by default")
	}
	if acc.Balance != 5000 {
		t.Fatalf("expected balance 5000 derived from postings, got %d", acc.Balance)
	}
}
