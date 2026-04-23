package idempotency

import (
	"testing"
	"time"

	"github.com/Godswilly/fingo/internal/errs"
)

func TestEvaluate_DecisionMatrix_TableDriven(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	cases := []struct {
		name         string
		existing     *Record
		incomingFP   string
		wantDecision Decision
		wantErr      bool
		wantSentinel error
		wantCode     errs.Code
	}{
		{
			name:         "no existing record executes",
			existing:     nil,
			incomingFP:   "fp-1",
			wantDecision: DecisionExecute,
		},
		{
			name: "same fingerprint completed replays",
			existing: &Record{
				Key:         Key("k1"),
				Fingerprint: "fp-1",
				Status:      StatusCompleted,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			incomingFP:   "fp-1",
			wantDecision: DecisionReplay,
		},
		{
			name: "same fingerprint in progress retries later",
			existing: &Record{
				Key:         Key("k1"),
				Fingerprint: "fp-1",
				Status:      StatusInProgress,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			incomingFP:   "fp-1",
			wantDecision: DecisionRetryLater,
			wantErr:      true,
			wantSentinel: ErrRequestInProgress,
			wantCode:     errs.CodeConflict,
		},
		{
			name: "same fingerprint failed executes again",
			existing: &Record{
				Key:         Key("k1"),
				Fingerprint: "fp-1",
				Status:      StatusFailed,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			incomingFP:   "fp-1",
			wantDecision: DecisionExecute,
		},
		{
			name: "different fingerprint is conflict",
			existing: &Record{
				Key:         Key("k1"),
				Fingerprint: "fp-1",
				Status:      StatusCompleted,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			incomingFP:   "fp-2",
			wantDecision: DecisionRejectConflict,
			wantErr:      true,
			wantSentinel: ErrKeyConflict,
			wantCode:     errs.CodeConflict,
		},
		{
			name:         "empty incoming fingerprint invalid",
			existing:     nil,
			incomingFP:   " ",
			wantDecision: DecisionRejectConflict,
			wantErr:      true,
			wantSentinel: ErrFingerprintRequired,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name: "unknown existing status is invariant violation",
			existing: &Record{
				Key:         Key("k1"),
				Fingerprint: "fp-1",
				Status:      Status("mystery"),
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			incomingFP:   "fp-1",
			wantDecision: DecisionRejectConflict,
			wantErr:      true,
			wantSentinel: ErrUnknownStatus,
			wantCode:     errs.CodeInvariantViolation,
		},
		{
			name: "empty existing fingerprint is invariant violation",
			existing: &Record{
				Key:         Key("k1"),
				Fingerprint: " ",
				Status:      StatusCompleted,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			incomingFP:   "fp-1",
			wantDecision: DecisionRejectConflict,
			wantErr:      true,
			wantSentinel: ErrFingerprintRequired,
			wantCode:     errs.CodeInvariantViolation,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotDecision, err := Evaluate(tc.existing, tc.incomingFP)
			if gotDecision != tc.wantDecision {
				t.Fatalf("unexpected decision: want %q, got %q", tc.wantDecision, gotDecision)
			}

			if tc.wantErr {
				assertDomainError(t, err, tc.wantSentinel, tc.wantCode)
				return
			}

			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}
