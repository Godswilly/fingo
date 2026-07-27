package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "github.com/Godswilly/fingo/internal/database/db"
	domainledger "github.com/Godswilly/fingo/internal/domain/ledger"
	"github.com/Godswilly/fingo/internal/errs"
)

// JournalStore implements internal/app/ledger.JournalStore.
type JournalStore struct {
	db *DB
}

func NewJournalStore(db *DB) *JournalStore {
	return &JournalStore{db: db}
}

// Save inserts the journal and its postings atomically. Callers are
// expected to invoke this within Transactor.WithinTx.
func (s *JournalStore) Save(ctx context.Context, entry *domainledger.JournalEntry) error {
	const op = "postgres.JournalStore.Save"

	q, err := s.db.txQueries(ctx, op)
	if err != nil {
		return err
	}

	metadata, err := json.Marshal(entry.Metadata())
	if err != nil {
		return errs.Infrastructure(errs.CodeInternal, op, "failed to encode journal metadata", err)
	}

	if err := q.InsertJournal(ctx, dbgen.InsertJournalParams{
		ID:          entry.ID(),
		Reference:   entry.Reference(),
		Description: entry.Description(),
		Metadata:    metadata,
		CreatedAt: pgtype.Timestamptz{
			Time:  entry.CreatedAt(),
			Valid: true,
		},
	}); err != nil {
		return wrapErr(op, err)
	}

	for i, posting := range entry.Postings() {
		if err := q.InsertPosting(ctx, dbgen.InsertPostingParams{
			JournalID:    entry.ID(),
			PostingIndex: int32(i),
			AccountID:    posting.AccountID(),
			Amount:       posting.Amount(),
			Side:         string(posting.Side()),
			Currency:     posting.Currency(),
		}); err != nil {
			return wrapErr(op, err)
		}
	}

	return nil
}

// Get reconstructs a domain JournalEntry from the journal and posting rows.
func (s *JournalStore) Get(ctx context.Context, id string) (*domainledger.JournalEntry, error) {
	const op = "postgres.JournalStore.Get"

	q := s.db.queriesForContext(ctx)
	journal, err := q.GetJournal(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, wrapErr(op, err)
	}

	var metadata map[string]string
	if err := json.Unmarshal(journal.Metadata, &metadata); err != nil {
		return nil, errs.Infrastructure(errs.CodeInternal, op, "failed to decode journal metadata", err)
	}

	rows, err := q.ListJournalPostings(ctx, id)
	if err != nil {
		return nil, wrapErr(op, err)
	}

	postings := make([]domainledger.Posting, 0, len(rows))
	for _, row := range rows {
		posting, err := domainledger.NewPosting(row.AccountID, row.Amount, domainledger.PostingSide(row.Side), row.Currency)
		if err != nil {
			return nil, errs.Infrastructure(errs.CodeInvariantViolation, op, "persisted posting failed domain validation", err)
		}
		postings = append(postings, posting)
	}

	entry, err := domainledger.NewJournalEntry(domainledger.CreateJournalEntryInput{
		ID:          journal.ID,
		Reference:   journal.Reference,
		Description: journal.Description,
		CreatedAt:   journalTime(journal.CreatedAt),
		Postings:    postings,
		Metadata:    metadata,
	})
	if err != nil {
		return nil, errs.Infrastructure(errs.CodeInvariantViolation, op, "persisted journal failed domain validation", err)
	}

	return entry, nil
}

func journalTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}
