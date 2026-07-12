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
	Operation     string
	AccountID     string
	FromAccountID string
	ToAccountID   string
	Currency      string
	Reference     string
	Amount        int64
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

	var canonical strings.Builder
	writeCanonicalField(&canonical, "operation", op)
	writeCanonicalField(&canonical, "account_id", strings.TrimSpace(cmd.AccountID))
	writeCanonicalField(&canonical, "from_account_id", strings.TrimSpace(cmd.FromAccountID))
	writeCanonicalField(&canonical, "to_account_id", strings.TrimSpace(cmd.ToAccountID))
	writeCanonicalField(&canonical, "amount", strconv.FormatInt(cmd.Amount, 10))
	writeCanonicalField(&canonical, "currency", strings.ToUpper(strings.TrimSpace(cmd.Currency)))
	writeCanonicalField(&canonical, "reference", strings.TrimSpace(cmd.Reference))

	sum := sha256.Sum256([]byte(canonical.String()))
	return hex.EncodeToString(sum[:]), nil
}

func writeCanonicalField(b *strings.Builder, name, value string) {
	b.WriteString(name)
	b.WriteByte('=')
	b.WriteString(strconv.Itoa(len(value)))
	b.WriteByte(':')
	b.WriteString(value)
	b.WriteByte('\n')
}
