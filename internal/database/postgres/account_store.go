package postgres

import (
	"context"

	"github.com/Godswilly/fingo/internal/domain/account"
)

// AccountStore implements internal/app/ledger.AccountStore.
type AccountStore struct {
	db *DB
}

func NewAccountStore(db *DB) *AccountStore {
	return &AccountStore{db: db}
}

// GetForUpdate locks the account identity row and returns a balance
// snapshot derived from ledger_postings committed within this transaction's
// view. Every writer locks the same row via this method before inserting
// postings for the account, which is what makes the aggregate consistent.
func (s *AccountStore) GetForUpdate(ctx context.Context, id string) (*account.Account, error) {
	const op = "postgres.AccountStore.GetForUpdate"

	q, err := s.db.txQueries(ctx, op)
	if err != nil {
		return nil, err
	}

	row, err := q.LockAccount(ctx, id)
	if err != nil {
		return nil, wrapErr(op, err)
	}

	balance, err := q.GetAccountBalance(ctx, id)
	if err != nil {
		return nil, wrapErr(op, err)
	}

	return &account.Account{
		ID:        row.ID,
		Owner:     row.Owner,
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt.Time,
		Balance:   balance,
	}, nil
}
