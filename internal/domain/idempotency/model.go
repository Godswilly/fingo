package idempotency

import (
	"errors"
	"strings"

	"github.com/Godswilly/fingo/internal/errs"
)

const (
	maxKeyLength = 128
	opNewKey     = "idempotency.NewKey"
)

var (
	ErrKeyRequired         = errors.New("idempotency key cannot be empty")
	ErrKeyTooLong          = errors.New("idempotency key exceeds maximum length")
	ErrFingerprintRequired = errors.New("fingerprint cannot be empty")
)

// Key is an immutable idempotency key value object.
type Key string

func NewKey(raw string) (Key, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", errs.Domain(
			errs.CodeInvalidArgument,
			opNewKey,
			"idempotency key cannot be empty",
			ErrKeyRequired,
		)
	}
	if len(normalized) > maxKeyLength {
		return "", errs.Domain(
			errs.CodeInvalidArgument,
			opNewKey,
			"idempotency key exceeds maximum length",
			ErrKeyTooLong,
		)
	}

	return Key(normalized), nil
}

func (k Key) String() string {
	return string(k)
}
