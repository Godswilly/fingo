package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"

	"github.com/Godswilly/fingo/internal/errs"
)

const opBuildFingerprint = "idempotency.BuildFingerprint"

var (
	ErrOperationRequired = errors.New("operation cannot be empty")
	ErrAmountInvalid     = errors.New("amount must be greater than zero")
)

// Command is the canonical input used to derive deterministic idempotency fingerprints.
type Command struct {
	Operation string
	AccountID string
	Reference string
	Amount    int64
}

func BuildFingerprint(cmd Command) (string, error) {
	op := strings.TrimSpace(cmd.Operation)
	if op == "" {
		return "", errs.Domain(
			errs.CodeInvalidArgument,
			opBuildFingerprint,
			"operation cannot be empty",
			ErrOperationRequired,
		)
	}
	if cmd.Amount <= 0 {
		return "", errs.Domain(
			errs.CodeInvalidArgument,
			opBuildFingerprint,
			"amount must be greater than zero",
			ErrAmountInvalid,
		)
	}

	canonical := op + "|" +
		strings.TrimSpace(cmd.AccountID) + "|" +
		strconv.FormatInt(cmd.Amount, 10) + "|" +
		strings.TrimSpace(cmd.Reference)

	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:]), nil
}
