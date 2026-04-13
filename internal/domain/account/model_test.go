package account

import (
	"errors"
	"testing"
	"time"

	"github.com/Godswilly/fingo/internal/errs"
)

type newAccountErrCase struct {
	name          string
	id            string
	owner         string
	initialAmount int64
	wantSentinel  error
	wantCode      errs.Code
}

type newAccountSuccessCase struct {
	name          string
	id            string
	owner         string
	initialAmount int64
}

func TestNewAccount_ValidationErrors_TableDriven(t *testing.T) {
	cases := []newAccountErrCase{
		{
			name:          "empty owner",
			id:            "acc-001",
			owner:         "",
			initialAmount: 1000,
			wantSentinel:  ErrInvalidOwner,
			wantCode:      errs.CodeInvalidArgument,
		},
		{
			name:          "negative balance",
			id:            "acc-002",
			owner:         "alice",
			initialAmount: -1,
			wantSentinel:  ErrInitialBalance,
			wantCode:      errs.CodeInvalidArgument,
		},
		{
			name:          "negative large balance",
			id:            "acc-003",
			owner:         "bob",
			initialAmount: -999_999,
			wantSentinel:  ErrInitialBalance,
			wantCode:      errs.CodeInvalidArgument,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAccount(tc.id, tc.owner, tc.initialAmount)
			assertDomainValidationError(t, err, tc.wantSentinel, tc.wantCode)
		})
	}
}

func TestNewAccount_Success_TableDriven(t *testing.T) {
	cases := []newAccountSuccessCase{
		{
			name:          "zero balance allowed",
			id:            "acc-100",
			owner:         "alice",
			initialAmount: 0,
		},
		{
			name:          "positive balance",
			id:            "acc-101",
			owner:         "bob",
			initialAmount: 1500,
		},
		{
			name:          "large positive balance",
			id:            "acc-102",
			owner:         "carol",
			initialAmount: 2_000_000_000,
		},
		{
			name:          "owner with spaces is accepted",
			id:            "acc-103",
			owner:         "Ada Lovelace",
			initialAmount: 42,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now()
			acc, err := NewAccount(tc.id, tc.owner, tc.initialAmount)
			if err != nil {
				t.Fatalf("expected success, got %v", err)
			}
			assertAccountState(t, acc, tc.id, tc.owner, tc.initialAmount)
			if acc.CreatedAt.Before(before.Add(-1 * time.Second)) {
				t.Fatalf("created_at is unexpectedly old: %v", acc.CreatedAt)
			}
		})
	}
}

func assertDomainValidationError(t *testing.T, err error, wantSentinel error, wantCode errs.Code) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantSentinel) {
		t.Fatalf("expected wrapped sentinel %v, got %v", wantSentinel, err)
	}
	if !errs.IsLayer(err, errs.LayerDomain) {
		t.Fatalf("expected domain-layer error, got %v", err)
	}
	if !errs.IsCode(err, wantCode) {
		t.Fatalf("expected error code %q, got %v", wantCode, err)
	}
}

func assertAccountState(t *testing.T, acc *Account, wantID, wantOwner string, wantBalance int64) {
	t.Helper()

	if acc == nil {
		t.Fatal("expected non-nil account")
	}
	if acc.ID != wantID {
		t.Fatalf("id mismatch: want %q, got %q", wantID, acc.ID)
	}
	if acc.Owner != wantOwner {
		t.Fatalf("owner mismatch: want %q, got %q", wantOwner, acc.Owner)
	}
	if acc.Balance != wantBalance {
		t.Fatalf("balance mismatch: want %d, got %d", wantBalance, acc.Balance)
	}
	if !acc.IsActive {
		t.Fatal("expected new account to be active")
	}
	if acc.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}
}
