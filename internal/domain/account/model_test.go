package account

import (
	"errors"
	"math"
	"strings"
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

type stateTransitionCase struct {
	name         string
	startActive  bool
	action       func(*Account) error
	wantActive   bool
	wantSentinel error
	wantCode     errs.Code
	expectErr    bool
}

type moneyOperationCase struct {
	name         string
	startBalance int64
	startActive  bool
	amount       int64
	op           func(*Account, int64) error
	wantBalance  int64
	wantSentinel error
	wantCode     errs.Code
	expectErr    bool
}

func TestNewAccount_ValidationErrors_TableDriven(t *testing.T) {
	cases := []newAccountErrCase{
		{
			name:          "empty id",
			id:            "",
			owner:         "alice",
			initialAmount: 1000,
			wantSentinel:  ErrInvalidID,
			wantCode:      errs.CodeInvalidArgument,
		},
		{
			name:          "whitespace id",
			id:            "   ",
			owner:         "alice",
			initialAmount: 1000,
			wantSentinel:  ErrInvalidID,
			wantCode:      errs.CodeInvalidArgument,
		},
		{
			name:          "empty owner",
			id:            "acc-001",
			owner:         "",
			initialAmount: 1000,
			wantSentinel:  ErrInvalidOwner,
			wantCode:      errs.CodeInvalidArgument,
		},
		{
			name:          "whitespace owner",
			id:            "acc-001",
			owner:         "   ",
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
		{
			name:          "input values are trimmed",
			id:            "  acc-104 ",
			owner:         "  dora  ",
			initialAmount: 99,
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

func TestAccountStateTransitions_TableDriven(t *testing.T) {
	cases := []stateTransitionCase{
		{
			name:        "deactivate active account succeeds",
			startActive: true,
			action:      (*Account).Deactivate,
			wantActive:  false,
		},
		{
			name:        "activate inactive account succeeds",
			startActive: false,
			action:      (*Account).Activate,
			wantActive:  true,
		},
		{
			name:         "activate active account fails",
			startActive:  true,
			action:       (*Account).Activate,
			wantActive:   true,
			wantSentinel: ErrAlreadyActive,
			wantCode:     errs.CodeConflict,
			expectErr:    true,
		},
		{
			name:         "deactivate inactive account fails",
			startActive:  false,
			action:       (*Account).Deactivate,
			wantActive:   false,
			wantSentinel: ErrAlreadyInactive,
			wantCode:     errs.CodeConflict,
			expectErr:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acc := &Account{
				ID:       "acc-state",
				Owner:    "alice",
				Balance:  100,
				IsActive: tc.startActive,
			}

			err := tc.action(acc)
			if tc.expectErr {
				assertDomainValidationError(t, err, tc.wantSentinel, tc.wantCode)
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if acc.IsActive != tc.wantActive {
				t.Fatalf("unexpected account active state: want %v, got %v", tc.wantActive, acc.IsActive)
			}
		})
	}
}

func TestAccountCredit_TableDriven(t *testing.T) {
	cases := []moneyOperationCase{
		{
			name:         "credit active account succeeds",
			startBalance: 100,
			startActive:  true,
			amount:       50,
			op:           (*Account).Credit,
			wantBalance:  150,
		},
		{
			name:         "credit inactive account fails",
			startBalance: 100,
			startActive:  false,
			amount:       50,
			op:           (*Account).Credit,
			wantBalance:  100,
			wantSentinel: ErrInactiveAccount,
			wantCode:     errs.CodeConflict,
			expectErr:    true,
		},
		{
			name:         "credit zero amount fails",
			startBalance: 100,
			startActive:  true,
			amount:       0,
			op:           (*Account).Credit,
			wantBalance:  100,
			wantSentinel: ErrInvalidAmount,
			wantCode:     errs.CodeInvalidArgument,
			expectErr:    true,
		},
		{
			name:         "credit negative amount fails",
			startBalance: 100,
			startActive:  true,
			amount:       -1,
			op:           (*Account).Credit,
			wantBalance:  100,
			wantSentinel: ErrInvalidAmount,
			wantCode:     errs.CodeInvalidArgument,
			expectErr:    true,
		},
		{
			name:         "credit overflow fails",
			startBalance: math.MaxInt64,
			startActive:  true,
			amount:       1,
			op:           (*Account).Credit,
			wantBalance:  math.MaxInt64,
			wantSentinel: ErrBalanceOverflow,
			wantCode:     errs.CodeInvariantViolation,
			expectErr:    true,
		},
	}

	runMoneyOperationCases(t, cases)
}

func TestAccountDebit_TableDriven(t *testing.T) {
	cases := []moneyOperationCase{
		{
			name:         "debit active account succeeds",
			startBalance: 100,
			startActive:  true,
			amount:       40,
			op:           (*Account).Debit,
			wantBalance:  60,
		},
		{
			name:         "debit inactive account fails",
			startBalance: 100,
			startActive:  false,
			amount:       40,
			op:           (*Account).Debit,
			wantBalance:  100,
			wantSentinel: ErrInactiveAccount,
			wantCode:     errs.CodeConflict,
			expectErr:    true,
		},
		{
			name:         "debit zero amount fails",
			startBalance: 100,
			startActive:  true,
			amount:       0,
			op:           (*Account).Debit,
			wantBalance:  100,
			wantSentinel: ErrInvalidAmount,
			wantCode:     errs.CodeInvalidArgument,
			expectErr:    true,
		},
		{
			name:         "debit insufficient funds fails",
			startBalance: 100,
			startActive:  true,
			amount:       101,
			op:           (*Account).Debit,
			wantBalance:  100,
			wantSentinel: ErrInsufficientFunds,
			wantCode:     errs.CodeInvariantViolation,
			expectErr:    true,
		},
	}

	runMoneyOperationCases(t, cases)
}

func TestCanMethodsConsistencyWithMutatingOperations(t *testing.T) {
	t.Run("can credit uses same validation as credit", func(t *testing.T) {
		cases := []moneyOperationCase{
			{
				name:         "inactive account",
				startBalance: 100,
				startActive:  false,
				amount:       10,
				op:           (*Account).Credit,
				wantSentinel: ErrInactiveAccount,
				wantCode:     errs.CodeConflict,
				expectErr:    true,
			},
			{
				name:         "invalid amount",
				startBalance: 100,
				startActive:  true,
				amount:       0,
				op:           (*Account).Credit,
				wantSentinel: ErrInvalidAmount,
				wantCode:     errs.CodeInvalidArgument,
				expectErr:    true,
			},
			{
				name:         "overflow",
				startBalance: math.MaxInt64,
				startActive:  true,
				amount:       1,
				op:           (*Account).Credit,
				wantSentinel: ErrBalanceOverflow,
				wantCode:     errs.CodeInvariantViolation,
				expectErr:    true,
			},
			{
				name:         "valid",
				startBalance: 100,
				startActive:  true,
				amount:       10,
				op:           (*Account).Credit,
				expectErr:    false,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				acc := &Account{ID: "acc-1", Owner: "alice", Balance: tc.startBalance, IsActive: tc.startActive}
				err := acc.CanCredit(tc.amount)
				assertValidationOutcome(t, err, tc.wantSentinel, tc.wantCode, tc.expectErr)
			})
		}
	})

	t.Run("can debit uses same validation as debit", func(t *testing.T) {
		cases := []moneyOperationCase{
			{
				name:         "inactive account",
				startBalance: 100,
				startActive:  false,
				amount:       10,
				op:           (*Account).Debit,
				wantSentinel: ErrInactiveAccount,
				wantCode:     errs.CodeConflict,
				expectErr:    true,
			},
			{
				name:         "invalid amount",
				startBalance: 100,
				startActive:  true,
				amount:       0,
				op:           (*Account).Debit,
				wantSentinel: ErrInvalidAmount,
				wantCode:     errs.CodeInvalidArgument,
				expectErr:    true,
			},
			{
				name:         "insufficient funds",
				startBalance: 100,
				startActive:  true,
				amount:       101,
				op:           (*Account).Debit,
				wantSentinel: ErrInsufficientFunds,
				wantCode:     errs.CodeInvariantViolation,
				expectErr:    true,
			},
			{
				name:         "valid",
				startBalance: 100,
				startActive:  true,
				amount:       50,
				op:           (*Account).Debit,
				expectErr:    false,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				acc := &Account{ID: "acc-2", Owner: "bob", Balance: tc.startBalance, IsActive: tc.startActive}
				err := acc.CanDebit(tc.amount)
				assertValidationOutcome(t, err, tc.wantSentinel, tc.wantCode, tc.expectErr)
			})
		}
	})
}

func runMoneyOperationCases(t *testing.T, cases []moneyOperationCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acc := &Account{
				ID:       "acc-money",
				Owner:    "alice",
				Balance:  tc.startBalance,
				IsActive: tc.startActive,
			}

			err := tc.op(acc, tc.amount)
			if tc.expectErr {
				assertDomainValidationError(t, err, tc.wantSentinel, tc.wantCode)
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if acc.Balance != tc.wantBalance {
				t.Fatalf("unexpected balance: want %d, got %d", tc.wantBalance, acc.Balance)
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

func assertValidationOutcome(t *testing.T, err error, wantSentinel error, wantCode errs.Code, expectErr bool) {
	t.Helper()

	if expectErr {
		assertDomainValidationError(t, err, wantSentinel, wantCode)
		return
	}
	if err != nil {
		t.Fatalf("expected no validation error, got %v", err)
	}
}

func assertAccountState(t *testing.T, acc *Account, wantID, wantOwner string, wantBalance int64) {
	t.Helper()

	if acc == nil {
		t.Fatal("expected non-nil account")
	}
	if acc.ID != trimSpace(wantID) {
		t.Fatalf("id mismatch: want %q, got %q", trimSpace(wantID), acc.ID)
	}
	if acc.Owner != trimSpace(wantOwner) {
		t.Fatalf("owner mismatch: want %q, got %q", trimSpace(wantOwner), acc.Owner)
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

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
