package idempotency

import (
	"testing"

	"github.com/Godswilly/fingo/internal/errs"
)

func TestBuildFingerprint_Deterministic(t *testing.T) {
	t.Parallel()

	cmd := Command{
		Operation: "ledger.transfer",
		AccountID: "acc-1",
		Amount:    1000,
		Reference: "ref-1",
	}

	fp1, err := BuildFingerprint(cmd)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	fp2, err := BuildFingerprint(cmd)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if fp1 != fp2 {
		t.Fatalf("expected deterministic fingerprint, got %q and %q", fp1, fp2)
	}
}

func TestBuildFingerprint_DifferentInput_ProducesDifferentHash(t *testing.T) {
	t.Parallel()

	base := Command{
		Operation: "ledger.transfer",
		AccountID: "acc-1",
		Amount:    1000,
		Reference: "ref-1",
	}
	other := base
	other.Amount = 2000

	fp1, err := BuildFingerprint(base)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	fp2, err := BuildFingerprint(other)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if fp1 == fp2 {
		t.Fatalf("expected different fingerprints for different commands")
	}
}

func TestBuildFingerprint_ValidationErrors_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		cmd          Command
		wantSentinel error
		wantCode     errs.Code
	}{
		{
			name: "missing operation",
			cmd: Command{
				Operation: " ",
				AccountID: "acc-1",
				Amount:    100,
			},
			wantSentinel: ErrOperationRequired,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name: "invalid amount",
			cmd: Command{
				Operation: "ledger.transfer",
				AccountID: "acc-1",
				Amount:    0,
			},
			wantSentinel: ErrAmountInvalid,
			wantCode:     errs.CodeInvalidArgument,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := BuildFingerprint(tc.cmd)
			assertDomainError(t, err, tc.wantSentinel, tc.wantCode)
		})
	}
}
