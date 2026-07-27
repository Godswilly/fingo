//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Godswilly/fingo/internal/database/postgres"
	"github.com/Godswilly/fingo/internal/domain/idempotency"
	"github.com/Godswilly/fingo/migrations"
)

const defaultTestDatabaseURL = "postgres://fingo:fingo@localhost:5432/fingo?sslmode=disable"

// transferFingerprintOperation must match the unexported transferOperation
// constant in internal/app/ledger/service.go; it is part of the stable
// fingerprint contract from docs/adr/ADR-002.
const transferFingerprintOperation = "ledger.transfer"

var testDB *postgres.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("FINGO_DATABASE_URL")
	if dsn == "" {
		dsn = defaultTestDatabaseURL
	}

	ctx := context.Background()

	db, err := postgres.Open(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open test database: %v\n", err)
		os.Exit(1)
	}

	if err := migrations.Run(ctx, db.Pool); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	testDB = db
	code := m.Run()
	db.Close()
	os.Exit(code)
}

func truncateAll(t *testing.T) {
	t.Helper()

	_, err := testDB.Pool.Exec(context.Background(),
		`TRUNCATE outbox_events, ledger_postings, ledger_journal, idempotency_keys, accounts RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate all: %v", err)
	}
}

func withinTx(t *testing.T, fn func(context.Context) error) {
	t.Helper()

	if err := postgres.NewTransactor(testDB).WithinTx(context.Background(), fn); err != nil {
		t.Fatalf("within tx: %v", err)
	}
}

func insertAccount(t *testing.T, id, owner string) {
	t.Helper()

	_, err := testDB.Pool.Exec(context.Background(),
		`INSERT INTO accounts (id, owner) VALUES ($1, $2)`, id, owner)
	if err != nil {
		t.Fatalf("insert account %s: %v", id, err)
	}
}

// seedBalance gives accountID an opening balance by posting a two-line
// journal against a shared clearing account, exercising the same
// ledger_postings path CreateTransfer uses rather than a stored balance
// column (accounts has none; balance is always derived from postings).
func seedBalance(t *testing.T, accountID string, amount int64, currency string) {
	t.Helper()

	const equityID = "seed-equity"
	ctx := context.Background()

	_, err := testDB.Pool.Exec(ctx,
		`INSERT INTO accounts (id, owner) VALUES ($1, 'seed') ON CONFLICT (id) DO NOTHING`, equityID)
	if err != nil {
		t.Fatalf("insert seed equity account: %v", err)
	}

	journalID := fmt.Sprintf("seed-%s-%d", accountID, amount)
	_, err = testDB.Pool.Exec(ctx,
		`INSERT INTO ledger_journal (id, reference, description, metadata, created_at)
		 VALUES ($1, $1, 'seed balance', '{}'::jsonb, now())`,
		journalID,
	)
	if err != nil {
		t.Fatalf("insert seed journal: %v", err)
	}

	_, err = testDB.Pool.Exec(ctx,
		`INSERT INTO ledger_postings (journal_id, posting_index, account_id, amount, side, currency)
		 VALUES ($1, 0, $2, $3, 'debit', $4), ($1, 1, $5, $3, 'credit', $4)`,
		journalID, equityID, amount, currency, accountID,
	)
	if err != nil {
		t.Fatalf("insert seed postings: %v", err)
	}
}

func transferFingerprint(t *testing.T, fromAccountID, toAccountID, currency, reference string, amount int64) string {
	t.Helper()

	fp, err := idempotency.BuildFingerprint(idempotency.Command{
		Operation:     transferFingerprintOperation,
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		Currency:      currency,
		Reference:     reference,
		Amount:        amount,
	})
	if err != nil {
		t.Fatalf("build fingerprint: %v", err)
	}
	return fp
}
