package errs

import (
	"errors"
	"net/http"
	"testing"
)

func TestErrorWrappingAndClassification(t *testing.T) {
	root := errors.New("root failure")
	err := Domain(CodeInvalidArgument, "account.New", "owner is required", root)

	if !errors.Is(err, root) {
		t.Fatalf("expected wrapped root error")
	}
	if !IsCode(err, CodeInvalidArgument) {
		t.Fatalf("expected code classification")
	}
	if !IsLayer(err, LayerDomain) {
		t.Fatalf("expected layer classification")
	}
}

func TestHTTPStatusMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "invalid argument",
			err:  Domain(CodeInvalidArgument, "x", "bad input", nil),
			want: http.StatusBadRequest,
		},
		{
			name: "invariant violation",
			err:  Domain(CodeInvariantViolation, "ledger.Post", "imbalanced posting set", nil),
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "conflict",
			err:  Application(CodeConflict, "transfer.Create", "duplicate request", nil),
			want: http.StatusConflict,
		},
		{
			name: "timeout",
			err:  Infrastructure(CodeTimeout, "postgres.Exec", "query timeout", nil),
			want: http.StatusGatewayTimeout,
		},
		{
			name: "untyped error defaults to 500",
			err:  errors.New("boom"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HTTPStatus(tc.err)
			if got != tc.want {
				t.Fatalf("expected status %d, got %d", tc.want, got)
			}
		})
	}
}
