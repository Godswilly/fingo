package account

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/Godswilly/fingo/internal/errs"
)

// Account represents a financial entity in the FinGo ledger.
type Account struct {
	CreatedAt time.Time
	Balance   int64
	ID        string
	Owner     string
	IsActive  bool
}

// Sentinel errors for the account domain.
var (
	ErrInvalidID         = errors.New("account id cannot be empty")
	ErrInvalidOwner      = errors.New("owner name cannot be empty")
	ErrInitialBalance    = errors.New("initial balance cannot be negative")
	ErrAlreadyActive     = errors.New("account is already active")
	ErrAlreadyInactive   = errors.New("account is already inactive")
	ErrInactiveAccount   = errors.New("account is inactive")
	ErrInvalidAmount     = errors.New("amount must be greater than zero")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrBalanceOverflow   = errors.New("balance overflow")
)

// NewAccount is a domain constructor that enforces our business invariants.
func NewAccount(id, owner string, initialBalance int64) (*Account, error) {
	trimmedID := strings.TrimSpace(id)
	trimmedOwner := strings.TrimSpace(owner)

	if trimmedID == "" {
		return nil, errs.Domain(
			errs.CodeInvalidArgument,
			"account.NewAccount",
			"account id cannot be empty",
			ErrInvalidID,
		)
	}

	if trimmedOwner == "" {
		return nil, errs.Domain(
			errs.CodeInvalidArgument,
			"account.NewAccount",
			"owner name cannot be empty",
			ErrInvalidOwner,
		)
	}

	if initialBalance < 0 {
		return nil, errs.Domain(
			errs.CodeInvalidArgument,
			"account.NewAccount",
			"initial balance cannot be negative",
			ErrInitialBalance,
		)
	}

	return &Account{
		ID:        trimmedID,
		Owner:     trimmedOwner,
		Balance:   initialBalance,
		IsActive:  true,
		CreatedAt: time.Now(),
	}, nil
}

// Activate transitions an account into active state.
func (a *Account) Activate() error {
	if a.IsActive {
		return errs.Domain(
			errs.CodeConflict,
			"account.Activate",
			"account is already active",
			ErrAlreadyActive,
		)
	}
	a.IsActive = true
	return nil
}

// Deactivate transitions an account into inactive state.
func (a *Account) Deactivate() error {
	if !a.IsActive {
		return errs.Domain(
			errs.CodeConflict,
			"account.Deactivate",
			"account is already inactive",
			ErrAlreadyInactive,
		)
	}
	a.IsActive = false
	return nil
}

// CanCredit checks whether credit can be applied without mutating state.
func (a *Account) CanCredit(amount int64) error {
	return a.validateCredit("account.CanCredit", amount)
}

// CanDebit checks whether debit can be applied without mutating state.
func (a *Account) CanDebit(amount int64) error {
	return a.validateDebit("account.CanDebit", amount)
}

// Credit increases account balance by a positive amount.
func (a *Account) Credit(amount int64) error {
	if err := a.validateCredit("account.Credit", amount); err != nil {
		return err
	}

	a.Balance += amount
	return nil
}

// Debit decreases account balance by a positive amount, if funds are sufficient.
func (a *Account) Debit(amount int64) error {
	if err := a.validateDebit("account.Debit", amount); err != nil {
		return err
	}

	a.Balance -= amount
	return nil
}

func (a *Account) validateCredit(op string, amount int64) error {
	if !a.IsActive {
		return errs.Domain(
			errs.CodeConflict,
			op,
			"cannot credit inactive account",
			ErrInactiveAccount,
		)
	}
	if amount <= 0 {
		return errs.Domain(
			errs.CodeInvalidArgument,
			op,
			"amount must be greater than zero",
			ErrInvalidAmount,
		)
	}
	if a.Balance > math.MaxInt64-amount {
		return errs.Domain(
			errs.CodeInvariantViolation,
			op,
			"credit would overflow account balance",
			ErrBalanceOverflow,
		)
	}
	return nil
}

func (a *Account) validateDebit(op string, amount int64) error {
	if !a.IsActive {
		return errs.Domain(
			errs.CodeConflict,
			op,
			"cannot debit inactive account",
			ErrInactiveAccount,
		)
	}
	if amount <= 0 {
		return errs.Domain(
			errs.CodeInvalidArgument,
			op,
			"amount must be greater than zero",
			ErrInvalidAmount,
		)
	}
	if a.Balance < amount {
		return errs.Domain(
			errs.CodeInvariantViolation,
			op,
			"insufficient funds for debit",
			ErrInsufficientFunds,
		)
	}
	return nil
}
