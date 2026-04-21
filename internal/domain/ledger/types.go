package ledger

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/Godswilly/fingo/internal/errs"
)

// Package ledger defines core domain entities for immutable, append-only ledger entries.
//
// Money conventions:
//  1. Amount is stored in minor units using int64 (e.g., cents, kobo).
//  2. Amount is always positive (> 0).
//  3. Direction is represented by PostingSide (debit/credit), not by signed amounts.
//
// Journal-level invariants:
//  1. A journal entry must contain at least two postings.
//  2. Sum(debits) must equal sum(credits).
//  3. All non-empty currencies in a posting set must match (single-currency entry).

// PostingSide represents the accounting side for a posting line.
type PostingSide string

const (
	SideDebit  PostingSide = "debit"
	SideCredit PostingSide = "credit"
)

var (
	ErrJournalIDRequired     = errors.New("journal entry id cannot be empty")
	ErrReferenceRequired     = errors.New("journal entry reference cannot be empty")
	ErrPostingRequired       = errors.New("journal entry must contain at least one posting")
	ErrAccountIDRequired     = errors.New("posting account id cannot be empty")
	ErrAmountMustBePositive  = errors.New("posting amount must be greater than zero")
	ErrInvalidPostingSide    = errors.New("posting side must be either debit or credit")
	ErrInvalidCurrency       = errors.New("posting currency must be a 3-letter code (A-Z)")
	ErrMetadataKeyRequired   = errors.New("journal entry metadata key cannot be empty")
	ErrTooFewPostings        = errors.New("journal entry must contain at least 2 postings")
	ErrUnbalancedPostings    = errors.New("journal entry postings must balance (debits == credits)")
	ErrPostingTotalsOverflow = errors.New("posting totals overflow int64 bounds")
	ErrMixedCurrencies       = errors.New("journal entry postings must use a single currency")
	ErrInvalidInvariantState = errors.New("invalid posting side encountered during invariant evaluation")
)

const (
	opNewPosting      = "ledger.NewPosting"
	opNewJournalEntry = "ledger.NewJournalEntry"
)

// Posting is an immutable debit/credit line for a specific account.
type Posting struct {
	accountID string
	amount    int64
	side      PostingSide
	currency  string
}

// NewPosting creates a validated immutable posting.
func NewPosting(accountID string, amount int64, side PostingSide, currency string) (Posting, error) {
	posting := Posting{
		accountID: strings.TrimSpace(accountID),
		amount:    amount,
		side:      side,
		currency:  normalizeCurrency(currency),
	}

	if err := posting.validate(opNewPosting); err != nil {
		return Posting{}, err
	}

	return posting, nil
}

func (p Posting) validate(op string) error {
	if p.accountID == "" {
		return errs.Domain(
			errs.CodeInvalidArgument,
			op,
			"posting account id cannot be empty",
			ErrAccountIDRequired,
		)
	}

	if p.amount <= 0 {
		return errs.Domain(
			errs.CodeInvalidArgument,
			op,
			"posting amount must be greater than zero",
			ErrAmountMustBePositive,
		)
	}

	if p.side != SideDebit && p.side != SideCredit {
		return errs.Domain(
			errs.CodeInvalidArgument,
			op,
			"posting side must be either debit or credit",
			ErrInvalidPostingSide,
		)
	}

	if p.currency != "" && !isValidCurrencyCode(p.currency) {
		return errs.Domain(
			errs.CodeInvalidArgument,
			op,
			"posting currency must be a 3-letter code (A-Z)",
			ErrInvalidCurrency,
		)
	}

	return nil
}

func (p Posting) AccountID() string {
	return p.accountID
}

func (p Posting) Amount() int64 {
	return p.amount
}

func (p Posting) Side() PostingSide {
	return p.side
}

func (p Posting) Currency() string {
	return p.currency
}

// CreateJournalEntryInput is the command model used before domain validation.
type CreateJournalEntryInput struct {
	CreatedAt   time.Time
	Postings    []Posting
	Metadata    map[string]string
	ID          string
	Reference   string
	Description string
}

// JournalEntry is an immutable append-only ledger transfer envelope.
type JournalEntry struct {
	createdAt   time.Time
	postings    []Posting
	metadata    map[string]string
	id          string
	reference   string
	description string
}

// NewJournalEntry constructs a validated immutable journal entry.
func NewJournalEntry(input CreateJournalEntryInput) (*JournalEntry, error) {
	trimmedID := strings.TrimSpace(input.ID)
	trimmedRef := strings.TrimSpace(input.Reference)
	trimmedDescription := strings.TrimSpace(input.Description)

	if trimmedID == "" {
		return nil, errs.Domain(
			errs.CodeInvalidArgument,
			opNewJournalEntry,
			"journal entry id cannot be empty",
			ErrJournalIDRequired,
		)
	}

	if trimmedRef == "" {
		return nil, errs.Domain(
			errs.CodeInvalidArgument,
			opNewJournalEntry,
			"journal entry reference cannot be empty",
			ErrReferenceRequired,
		)
	}

	metadata := make(map[string]string, len(input.Metadata))
	for key, value := range input.Metadata {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return nil, errs.Domain(
				errs.CodeInvalidArgument,
				opNewJournalEntry,
				"journal entry metadata key cannot be empty",
				ErrMetadataKeyRequired,
			)
		}
		metadata[trimmedKey] = strings.TrimSpace(value)
	}

	postings := make([]Posting, len(input.Postings))
	copy(postings, input.Postings)
	for _, posting := range postings {
		if err := posting.validate(opNewJournalEntry); err != nil {
			return nil, err
		}
	}
	if err := validatePostingSetInvariants(opNewJournalEntry, postings); err != nil {
		return nil, err
	}

	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	createdAt = createdAt.UTC()

	return &JournalEntry{
		id:          trimmedID,
		reference:   trimmedRef,
		description: trimmedDescription,
		createdAt:   createdAt,
		postings:    postings,
		metadata:    metadata,
	}, nil
}

func normalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func isValidCurrencyCode(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, r := range currency {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func validatePostingSetInvariants(op string, postings []Posting) error {
	if len(postings) < 2 {
		return errs.Domain(
			errs.CodeInvariantViolation,
			op,
			"journal entry must contain at least 2 postings",
			ErrTooFewPostings,
		)
	}

	var debitTotal int64
	var creditTotal int64
	var expectedCurrency string
	for _, posting := range postings {
		if posting.currency != "" {
			if expectedCurrency == "" {
				expectedCurrency = posting.currency
			} else if posting.currency != expectedCurrency {
				return errs.Domain(
					errs.CodeInvariantViolation,
					op,
					"journal entry postings must use a single currency",
					ErrMixedCurrencies,
				)
			}
		}

		switch posting.side {
		case SideDebit:
			if debitTotal > math.MaxInt64-posting.amount {
				return errs.Domain(
					errs.CodeInvariantViolation,
					op,
					"debit total overflow while validating posting set",
					ErrPostingTotalsOverflow,
				)
			}
			debitTotal += posting.amount
		case SideCredit:
			if creditTotal > math.MaxInt64-posting.amount {
				return errs.Domain(
					errs.CodeInvariantViolation,
					op,
					"credit total overflow while validating posting set",
					ErrPostingTotalsOverflow,
				)
			}
			creditTotal += posting.amount
		default:
			return errs.Domain(
				errs.CodeInvariantViolation,
				op,
				"invalid posting side encountered during invariant evaluation",
				ErrInvalidInvariantState,
			)
		}
	}

	if debitTotal != creditTotal {
		return errs.Domain(
			errs.CodeInvariantViolation,
			op,
			"journal entry postings must balance (debits == credits)",
			ErrUnbalancedPostings,
		)
	}

	return nil
}

func (j *JournalEntry) ID() string {
	return j.id
}

func (j *JournalEntry) Reference() string {
	return j.reference
}

func (j *JournalEntry) Description() string {
	return j.description
}

func (j *JournalEntry) CreatedAt() time.Time {
	return j.createdAt
}

func (j *JournalEntry) Postings() []Posting {
	postings := make([]Posting, len(j.postings))
	copy(postings, j.postings)
	return postings
}

func (j *JournalEntry) Metadata() map[string]string {
	metadata := make(map[string]string, len(j.metadata))
	for key, value := range j.metadata {
		metadata[key] = value
	}
	return metadata
}
