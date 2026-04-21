package ledger

import (
	"errors"
	"testing"
	"time"

	"github.com/Godswilly/fingo/internal/errs"
)

func TestNewPosting_ValidationErrors_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		accountID    string
		amount       int64
		side         PostingSide
		currency     string
		wantSentinel error
		wantCode     errs.Code
	}{
		{
			name:         "empty account id",
			accountID:    "",
			amount:       100,
			side:         SideDebit,
			wantSentinel: ErrAccountIDRequired,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name:         "non positive amount",
			accountID:    "acc-1",
			amount:       0,
			side:         SideDebit,
			wantSentinel: ErrAmountMustBePositive,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name:         "invalid posting side",
			accountID:    "acc-1",
			amount:       100,
			side:         PostingSide("sideways"),
			wantSentinel: ErrInvalidPostingSide,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name:         "currency must be 3 letters",
			accountID:    "acc-1",
			amount:       100,
			side:         SideCredit,
			currency:     "USDT",
			wantSentinel: ErrInvalidCurrency,
			wantCode:     errs.CodeInvalidArgument,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewPosting(tc.accountID, tc.amount, tc.side, tc.currency)
			assertDomainError(t, err, tc.wantSentinel, tc.wantCode)
		})
	}
}

func TestNewPosting_SuccessAndNormalization(t *testing.T) {
	t.Parallel()

	p, err := NewPosting("  acc-123 ", 500, SideDebit, " usd ")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if p.AccountID() != "acc-123" {
		t.Fatalf("unexpected account id: %q", p.AccountID())
	}
	if p.Amount() != 500 {
		t.Fatalf("unexpected amount: %d", p.Amount())
	}
	if p.Side() != SideDebit {
		t.Fatalf("unexpected side: %q", p.Side())
	}
	if p.Currency() != "USD" {
		t.Fatalf("unexpected currency: %q", p.Currency())
	}
}

func TestNewJournalEntry_ValidationErrors_TableDriven(t *testing.T) {
	t.Parallel()

	validPosting, err := NewPosting("acc-1", 100, SideDebit, "")
	if err != nil {
		t.Fatalf("setup posting: %v", err)
	}
	validCredit, err := NewPosting("acc-2", 100, SideCredit, "")
	if err != nil {
		t.Fatalf("setup posting: %v", err)
	}

	cases := []struct {
		name         string
		input        CreateJournalEntryInput
		wantSentinel error
		wantCode     errs.Code
	}{
		{
			name: "empty journal id",
			input: CreateJournalEntryInput{
				ID:        "",
				Reference: "ref-1",
				Postings:  []Posting{validPosting, validCredit},
			},
			wantSentinel: ErrJournalIDRequired,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name: "empty reference",
			input: CreateJournalEntryInput{
				ID:        "jrn-1",
				Reference: " ",
				Postings:  []Posting{validPosting, validCredit},
			},
			wantSentinel: ErrReferenceRequired,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name: "no postings",
			input: CreateJournalEntryInput{
				ID:        "jrn-1",
				Reference: "ref-1",
				Postings:  nil,
			},
			wantSentinel: ErrTooFewPostings,
			wantCode:     errs.CodeInvariantViolation,
		},
		{
			name: "empty metadata key",
			input: CreateJournalEntryInput{
				ID:        "jrn-1",
				Reference: "ref-1",
				Postings:  []Posting{validPosting, validCredit},
				Metadata: map[string]string{
					" ": "value",
				},
			},
			wantSentinel: ErrMetadataKeyRequired,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name: "contains invalid posting",
			input: CreateJournalEntryInput{
				ID:        "jrn-1",
				Reference: "ref-1",
				Postings: []Posting{
					{
						accountID: "acc-1",
						amount:    100,
						side:      PostingSide("invalid"),
					},
				},
			},
			wantSentinel: ErrInvalidPostingSide,
			wantCode:     errs.CodeInvalidArgument,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, gotErr := NewJournalEntry(tc.input)
			assertDomainError(t, gotErr, tc.wantSentinel, tc.wantCode)
		})
	}
}

func TestNewJournalEntry_ImmutableDataAndDefaults(t *testing.T) {
	t.Parallel()

	debit, err := NewPosting("acc-1", 100, SideDebit, "USD")
	if err != nil {
		t.Fatalf("setup debit: %v", err)
	}
	credit, err := NewPosting("acc-2", 100, SideCredit, "USD")
	if err != nil {
		t.Fatalf("setup credit: %v", err)
	}

	input := CreateJournalEntryInput{
		ID:        "  jrn-001 ",
		Reference: " ref-001 ",
		Postings:  []Posting{debit, credit},
		Metadata: map[string]string{
			"request_id": " req-1 ",
		},
		Description: " transfer ",
	}

	entry, err := NewJournalEntry(input)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if entry.ID() != "jrn-001" {
		t.Fatalf("unexpected id: %q", entry.ID())
	}
	if entry.Reference() != "ref-001" {
		t.Fatalf("unexpected reference: %q", entry.Reference())
	}
	if entry.Description() != "transfer" {
		t.Fatalf("unexpected description: %q", entry.Description())
	}
	if entry.CreatedAt().IsZero() {
		t.Fatalf("createdAt should be set when zero input is provided")
	}

	// Mutate original input and confirm entry internals are isolated.
	input.Postings[0] = credit
	input.Metadata["request_id"] = "changed"

	gotPostings := entry.Postings()
	if gotPostings[0].AccountID() != "acc-1" {
		t.Fatalf("postings should be immutable from input mutation")
	}
	if entry.Metadata()["request_id"] != "req-1" {
		t.Fatalf("metadata should be immutable from input mutation")
	}

	// Mutate accessor return values and confirm internal state remains unchanged.
	gotPostings[0] = credit
	meta := entry.Metadata()
	meta["request_id"] = "changed-again"

	gotPostingsAgain := entry.Postings()
	if gotPostingsAgain[0].AccountID() != "acc-1" {
		t.Fatalf("postings should be immutable from accessor mutation")
	}
	if entry.Metadata()["request_id"] != "req-1" {
		t.Fatalf("metadata should be immutable from accessor mutation")
	}
}

func TestNewJournalEntry_UsesProvidedCreatedAt(t *testing.T) {
	t.Parallel()

	debit, err := NewPosting("acc-1", 100, SideDebit, "")
	if err != nil {
		t.Fatalf("setup posting: %v", err)
	}
	credit, err := NewPosting("acc-2", 100, SideCredit, "")
	if err != nil {
		t.Fatalf("setup posting: %v", err)
	}

	lagos := time.FixedZone("WAT", 1*60*60)
	expectedTime := time.Date(2026, 4, 21, 10, 0, 0, 0, lagos)
	entry, err := NewJournalEntry(CreateJournalEntryInput{
		ID:        "jrn-1",
		Reference: "ref-1",
		CreatedAt: expectedTime,
		Postings:  []Posting{debit, credit},
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	wantUTC := expectedTime.UTC()
	if !entry.CreatedAt().Equal(wantUTC) {
		t.Fatalf("expected createdAt %v, got %v", wantUTC, entry.CreatedAt())
	}
	if entry.CreatedAt().Location() != time.UTC {
		t.Fatalf("expected createdAt location UTC, got %v", entry.CreatedAt().Location())
	}
}

func TestNewJournalEntry_InvariantViolations_TableDriven(t *testing.T) {
	t.Parallel()

	debit100, err := NewPosting("acc-1", 100, SideDebit, "USD")
	if err != nil {
		t.Fatalf("setup debit: %v", err)
	}
	credit100, err := NewPosting("acc-2", 100, SideCredit, "USD")
	if err != nil {
		t.Fatalf("setup credit: %v", err)
	}
	credit90, err := NewPosting("acc-2", 90, SideCredit, "USD")
	if err != nil {
		t.Fatalf("setup credit: %v", err)
	}
	debitMax, err := NewPosting("acc-1", 9_223_372_036_854_775_000, SideDebit, "USD")
	if err != nil {
		t.Fatalf("setup debit max: %v", err)
	}
	debitLarge, err := NewPosting("acc-3", 900, SideDebit, "USD")
	if err != nil {
		t.Fatalf("setup debit large: %v", err)
	}
	creditMax, err := NewPosting("acc-2", 9_223_372_036_854_775_000, SideCredit, "USD")
	if err != nil {
		t.Fatalf("setup credit max: %v", err)
	}
	creditLarge, err := NewPosting("acc-4", 900, SideCredit, "USD")
	if err != nil {
		t.Fatalf("setup credit large: %v", err)
	}

	cases := []struct {
		name         string
		postings     []Posting
		wantSentinel error
		wantCode     errs.Code
	}{
		{
			name:         "too few postings",
			postings:     []Posting{debit100},
			wantSentinel: ErrTooFewPostings,
			wantCode:     errs.CodeInvariantViolation,
		},
		{
			name:         "unbalanced postings",
			postings:     []Posting{debit100, credit90},
			wantSentinel: ErrUnbalancedPostings,
			wantCode:     errs.CodeInvariantViolation,
		},
		{
			name:         "debit total overflow",
			postings:     []Posting{debitMax, debitLarge, credit100},
			wantSentinel: ErrPostingTotalsOverflow,
			wantCode:     errs.CodeInvariantViolation,
		},
		{
			name:         "credit total overflow",
			postings:     []Posting{creditMax, creditLarge, debit100},
			wantSentinel: ErrPostingTotalsOverflow,
			wantCode:     errs.CodeInvariantViolation,
		},
		{
			name: "mixed currencies are rejected",
			postings: []Posting{
				mustPosting(t, "acc-1", 100, SideDebit, "USD"),
				mustPosting(t, "acc-2", 100, SideCredit, "EUR"),
			},
			wantSentinel: ErrMixedCurrencies,
			wantCode:     errs.CodeInvariantViolation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, gotErr := NewJournalEntry(CreateJournalEntryInput{
				ID:        "jrn-inv-1",
				Reference: "ref-inv-1",
				Postings:  tc.postings,
			})
			assertDomainError(t, gotErr, tc.wantSentinel, tc.wantCode)
		})
	}
}

func TestNewJournalEntry_InvariantSuccessCases_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		postings []Posting
	}{
		{
			name: "balanced 3+ postings",
			postings: []Posting{
				mustPosting(t, "cash", 70, SideDebit, "USD"),
				mustPosting(t, "fees", 30, SideDebit, "USD"),
				mustPosting(t, "liability", 100, SideCredit, "USD"),
			},
		},
		{
			name: "same account can appear multiple times",
			postings: []Posting{
				mustPosting(t, "acc-1", 20, SideDebit, "USD"),
				mustPosting(t, "acc-1", 80, SideDebit, "USD"),
				mustPosting(t, "acc-2", 100, SideCredit, "USD"),
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			entry, err := NewJournalEntry(CreateJournalEntryInput{
				ID:        "jrn-ok-1",
				Reference: "ref-ok-1",
				Postings:  tc.postings,
			})
			if err != nil {
				t.Fatalf("expected success, got %v", err)
			}
			if len(entry.Postings()) != len(tc.postings) {
				t.Fatalf("unexpected posting count: want %d, got %d", len(tc.postings), len(entry.Postings()))
			}
		})
	}
}

func mustPosting(t *testing.T, accountID string, amount int64, side PostingSide, currency string) Posting {
	t.Helper()
	p, err := NewPosting(accountID, amount, side, currency)
	if err != nil {
		t.Fatalf("setup posting failed: %v", err)
	}
	return p
}

func assertDomainError(t *testing.T, err error, wantSentinel error, wantCode errs.Code) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, wantSentinel) {
		t.Fatalf("expected sentinel %v, got %v", wantSentinel, err)
	}
	if !errs.IsCode(err, wantCode) {
		t.Fatalf("expected code %q, got %v", wantCode, err)
	}
}
