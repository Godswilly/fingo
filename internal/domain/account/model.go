package account

import (
	"errors"
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
	ErrInvalidOwner   = errors.New("owner name cannot be empty")
	ErrInitialBalance = errors.New("initial balance cannot be negative")
)

// NewAccount is a domain constructor that enforces our business invariants.
func NewAccount(id, owner string, initialBalance int64) (*Account, error) {
	if owner == "" {
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
		ID:        id,
		Owner:     owner,
		Balance:   initialBalance,
		IsActive:  true,
		CreatedAt: time.Now(),
	}, nil
}
