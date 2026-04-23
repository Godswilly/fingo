package idempotency

import (
	"errors"
	"strings"
	"time"

	"github.com/Godswilly/fingo/internal/errs"
)

const (
	opEvaluate = "idempotency.Evaluate"
)

var (
	ErrKeyConflict       = errors.New("idempotency key already used for different command")
	ErrRequestInProgress = errors.New("request with this idempotency key is in progress")
	ErrUnknownStatus     = errors.New("unknown idempotency record status")
)

type Status string

const (
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

type Decision string

const (
	DecisionExecute        Decision = "execute"
	DecisionReplay         Decision = "replay"
	DecisionRetryLater     Decision = "retry_later"
	DecisionRejectConflict Decision = "reject_conflict"
)

// Record represents persisted idempotency state for a key.
type Record struct {
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Key         Key
	Fingerprint string
	Status      Status
	ResultRef   string
}

func Evaluate(existing *Record, incomingFingerprint string) (Decision, error) {
	fp := strings.TrimSpace(incomingFingerprint)
	if fp == "" {
		return DecisionRejectConflict, errs.Domain(
			errs.CodeInvalidArgument,
			opEvaluate,
			"fingerprint cannot be empty",
			ErrFingerprintRequired,
		)
	}

	if existing == nil {
		return DecisionExecute, nil
	}

	existingFP := strings.TrimSpace(existing.Fingerprint)
	if existingFP == "" {
		return DecisionRejectConflict, errs.Domain(
			errs.CodeInvariantViolation,
			opEvaluate,
			"existing idempotency record has empty fingerprint",
			ErrFingerprintRequired,
		)
	}

	if existingFP != fp {
		return DecisionRejectConflict, errs.Domain(
			errs.CodeConflict,
			opEvaluate,
			"idempotency key already used for different command",
			ErrKeyConflict,
		)
	}

	switch existing.Status {
	case StatusCompleted:
		return DecisionReplay, nil
	case StatusInProgress:
		return DecisionRetryLater, errs.Domain(
			errs.CodeConflict,
			opEvaluate,
			"request with this idempotency key is in progress",
			ErrRequestInProgress,
		)
	case StatusFailed:
		return DecisionExecute, nil
	default:
		return DecisionRejectConflict, errs.Domain(
			errs.CodeInvariantViolation,
			opEvaluate,
			"unknown idempotency record status",
			ErrUnknownStatus,
		)
	}
}
