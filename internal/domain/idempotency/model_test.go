package idempotency

import (
	"errors"
	"strings"
	"testing"

	"github.com/Godswilly/fingo/internal/errs"
)

func TestNewKey_ValidationErrors_TableDriven(t *testing.T) {
	t.Parallel()

	tooLong := strings.Repeat("a", maxKeyLength+1)
	cases := []struct {
		name         string
		raw          string
		wantSentinel error
		wantCode     errs.Code
	}{
		{
			name:         "empty key",
			raw:          "",
			wantSentinel: ErrKeyRequired,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name:         "whitespace key",
			raw:          "   ",
			wantSentinel: ErrKeyRequired,
			wantCode:     errs.CodeInvalidArgument,
		},
		{
			name:         "too long",
			raw:          tooLong,
			wantSentinel: ErrKeyTooLong,
			wantCode:     errs.CodeInvalidArgument,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewKey(tc.raw)
			assertDomainError(t, err, tc.wantSentinel, tc.wantCode)
		})
	}
}

func TestNewKey_Success_Normalization(t *testing.T) {
	t.Parallel()

	key, err := NewKey("  req-123  ")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if key.String() != "req-123" {
		t.Fatalf("unexpected key value: %q", key.String())
	}
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
