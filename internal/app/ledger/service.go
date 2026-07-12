package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Godswilly/fingo/internal/domain/account"
	"github.com/Godswilly/fingo/internal/domain/idempotency"
	domainledger "github.com/Godswilly/fingo/internal/domain/ledger"
	"github.com/Godswilly/fingo/internal/errs"
)

const (
	opNewTransferService = "ledger.NewTransferService"
	opCreateTransfer     = "ledger.TransferService.CreateTransfer"
	transferOperation    = "ledger.transfer"
)

var (
	ErrMissingDependency   = errors.New("missing transfer service dependency")
	ErrInvalidCommand      = errors.New("invalid transfer command")
	ErrSameAccount         = errors.New("source and destination accounts must differ")
	ErrAccountNotFound     = errors.New("account not found")
	ErrReplayResultMissing = errors.New("completed idempotency record is missing result reference")
	ErrReservationRace     = errors.New("idempotency reservation race")
)

// CreateTransferCommand is the application-level command for moving funds
// between two ledger accounts.
type CreateTransferCommand struct {
	CreatedAt      time.Time
	Metadata       map[string]string
	ID             string
	IdempotencyKey string
	Reference      string
	Description    string
	FromAccountID  string
	ToAccountID    string
	Currency       string
	Amount         int64
}

// CreateTransferResult is the stable application result for a transfer command.
type CreateTransferResult struct {
	CreatedAt      time.Time
	JournalID      string
	Reference      string
	IdempotencyKey string
	Replayed       bool
}

// TransferServiceDeps groups TransferService collaborators.
type TransferServiceDeps struct {
	Transactor    Transactor
	Accounts      AccountStore
	Journals      JournalStore
	Idempotencies IdempotencyStore
	Outbox        OutboxStore
}

// TransferService coordinates transfer creation across domain objects and ports.
type TransferService struct {
	tx            Transactor
	accounts      AccountStore
	journals      JournalStore
	idempotencies IdempotencyStore
	outbox        OutboxStore
}

func NewTransferService(deps TransferServiceDeps) (*TransferService, error) {
	if deps.Transactor == nil {
		return nil, missingDependency("transactor")
	}
	if deps.Accounts == nil {
		return nil, missingDependency("accounts")
	}
	if deps.Journals == nil {
		return nil, missingDependency("journals")
	}
	if deps.Idempotencies == nil {
		return nil, missingDependency("idempotencies")
	}
	if deps.Outbox == nil {
		return nil, missingDependency("outbox")
	}

	return &TransferService{
		tx:            deps.Transactor,
		accounts:      deps.Accounts,
		journals:      deps.Journals,
		idempotencies: deps.Idempotencies,
		outbox:        deps.Outbox,
	}, nil
}

func (s *TransferService) CreateTransfer(ctx context.Context, cmd CreateTransferCommand) (CreateTransferResult, error) {
	normalized, key, err := normalizeCommand(cmd)
	if err != nil {
		return CreateTransferResult{}, err
	}

	fingerprint, err := idempotency.BuildFingerprint(idempotency.Command{
		Operation:     transferOperation,
		FromAccountID: normalized.FromAccountID,
		ToAccountID:   normalized.ToAccountID,
		Currency:      normalized.Currency,
		Reference:     normalized.Reference,
		Amount:        normalized.Amount,
	})
	if err != nil {
		return CreateTransferResult{}, err
	}

	var result CreateTransferResult
	err = s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		existing, reserved, err := s.idempotencies.Reserve(txCtx, key, fingerprint)
		if err != nil {
			return err
		}

		if !reserved {
			shouldExecute, err := s.handleExistingReservation(txCtx, key, fingerprint, existing, normalized.Reference, &result)
			if err != nil {
				return err
			}
			if !shouldExecute {
				return nil
			}
		}

		from, to, err := s.loadAccountsForTransfer(txCtx, normalized.FromAccountID, normalized.ToAccountID)
		if err != nil {
			return err
		}
		if err := from.CanDebit(normalized.Amount); err != nil {
			return err
		}
		if err := to.CanCredit(normalized.Amount); err != nil {
			return err
		}

		entry, err := buildJournalEntry(normalized)
		if err != nil {
			return err
		}
		if err := s.journals.Save(txCtx, entry); err != nil {
			return err
		}
		if err := s.idempotencies.SaveCompleted(txCtx, key, entry.ID()); err != nil {
			return err
		}
		if err := s.outbox.Save(txCtx, buildTransferCreatedEvent(entry, normalized)); err != nil {
			return err
		}

		result = CreateTransferResult{
			JournalID:      entry.ID(),
			Reference:      entry.Reference(),
			IdempotencyKey: key.String(),
			CreatedAt:      entry.CreatedAt(),
		}
		return nil
	})
	if err != nil {
		return CreateTransferResult{}, err
	}

	return result, nil
}

func (s *TransferService) handleExistingReservation(
	ctx context.Context,
	key idempotency.Key,
	fingerprint string,
	existing *idempotency.Record,
	reference string,
	result *CreateTransferResult,
) (bool, error) {
	decision, err := idempotency.Evaluate(existing, fingerprint)
	switch decision {
	case idempotency.DecisionReplay:
		replay, err := s.replayResult(ctx, existing, key)
		if err != nil {
			return false, err
		}
		*result = replay
		return false, nil
	case idempotency.DecisionRetryLater, idempotency.DecisionRejectConflict:
		return false, wrapIdempotencyDecisionError(decision, err)
	case idempotency.DecisionExecute:
		if err != nil {
			return false, wrapIdempotencyDecisionError(decision, err)
		}
		ok, current, err := s.idempotencies.MarkInProgress(ctx, key, fingerprint)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
		return s.handleLostRetryClaim(ctx, key, fingerprint, current, reference, result)
	default:
		return false, unknownIdempotencyDecisionError()
	}
}

func (s *TransferService) handleLostRetryClaim(
	ctx context.Context,
	key idempotency.Key,
	fingerprint string,
	current *idempotency.Record,
	reference string,
	result *CreateTransferResult,
) (bool, error) {
	decision, err := idempotency.Evaluate(current, fingerprint)
	switch decision {
	case idempotency.DecisionReplay:
		replay, err := s.replayResult(ctx, current, key)
		if err != nil {
			return false, err
		}
		*result = replay
		return false, nil
	case idempotency.DecisionRetryLater, idempotency.DecisionRejectConflict:
		return false, wrapIdempotencyDecisionError(decision, err)
	case idempotency.DecisionExecute:
		if err != nil {
			return false, wrapIdempotencyDecisionError(decision, err)
		}
		return false, reservationRaceError(reference)
	default:
		return false, unknownIdempotencyDecisionError()
	}
}

func normalizeCommand(cmd CreateTransferCommand) (CreateTransferCommand, idempotency.Key, error) {
	normalized := CreateTransferCommand{
		CreatedAt:      cmd.CreatedAt,
		Metadata:       cmd.Metadata,
		ID:             strings.TrimSpace(cmd.ID),
		IdempotencyKey: strings.TrimSpace(cmd.IdempotencyKey),
		Reference:      strings.TrimSpace(cmd.Reference),
		Description:    strings.TrimSpace(cmd.Description),
		FromAccountID:  strings.TrimSpace(cmd.FromAccountID),
		ToAccountID:    strings.TrimSpace(cmd.ToAccountID),
		Currency:       strings.ToUpper(strings.TrimSpace(cmd.Currency)),
		Amount:         cmd.Amount,
	}

	if normalized.ID == "" {
		return CreateTransferCommand{}, "", invalidCommand("transfer id cannot be empty", ErrInvalidCommand)
	}
	if normalized.FromAccountID == "" {
		return CreateTransferCommand{}, "", invalidCommand("source account id cannot be empty", ErrInvalidCommand)
	}
	if normalized.ToAccountID == "" {
		return CreateTransferCommand{}, "", invalidCommand("destination account id cannot be empty", ErrInvalidCommand)
	}
	if normalized.Reference == "" {
		return CreateTransferCommand{}, "", invalidCommand("transfer reference cannot be empty", ErrInvalidCommand)
	}
	if normalized.FromAccountID == normalized.ToAccountID {
		return CreateTransferCommand{}, "", invalidCommand("source and destination accounts must differ", ErrSameAccount)
	}

	key, err := idempotency.NewKey(normalized.IdempotencyKey)
	if err != nil {
		return CreateTransferCommand{}, "", err
	}

	return normalized, key, nil
}

func (s *TransferService) loadAccountsForTransfer(ctx context.Context, fromID, toID string) (*account.Account, *account.Account, error) {
	if fromID < toID {
		from, err := s.getAccountForUpdate(ctx, fromID)
		if err != nil {
			return nil, nil, err
		}
		to, err := s.getAccountForUpdate(ctx, toID)
		if err != nil {
			return nil, nil, err
		}
		return from, to, nil
	}

	to, err := s.getAccountForUpdate(ctx, toID)
	if err != nil {
		return nil, nil, err
	}
	from, err := s.getAccountForUpdate(ctx, fromID)
	if err != nil {
		return nil, nil, err
	}
	return from, to, nil
}

func (s *TransferService) getAccountForUpdate(ctx context.Context, id string) (*account.Account, error) {
	acc, err := s.accounts.GetForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, errs.Application(
			errs.CodeNotFound,
			opCreateTransfer,
			"account not found",
			fmt.Errorf("%w: %s", ErrAccountNotFound, id),
		)
	}
	return acc, nil
}

func buildJournalEntry(cmd CreateTransferCommand) (*domainledger.JournalEntry, error) {
	sourcePosting, err := domainledger.NewPosting(cmd.FromAccountID, cmd.Amount, domainledger.SideDebit, cmd.Currency)
	if err != nil {
		return nil, err
	}
	destinationPosting, err := domainledger.NewPosting(cmd.ToAccountID, cmd.Amount, domainledger.SideCredit, cmd.Currency)
	if err != nil {
		return nil, err
	}

	return domainledger.NewJournalEntry(domainledger.CreateJournalEntryInput{
		ID:          cmd.ID,
		Reference:   cmd.Reference,
		Description: cmd.Description,
		CreatedAt:   cmd.CreatedAt,
		Postings:    []domainledger.Posting{sourcePosting, destinationPosting},
		Metadata:    cmd.Metadata,
	})
}

func (s *TransferService) replayResult(ctx context.Context, record *idempotency.Record, key idempotency.Key) (CreateTransferResult, error) {
	if record == nil || strings.TrimSpace(record.ResultRef) == "" {
		return CreateTransferResult{}, errs.Application(
			errs.CodeInvariantViolation,
			opCreateTransfer,
			"completed idempotency record is missing result reference",
			ErrReplayResultMissing,
		)
	}

	entry, err := s.journals.Get(ctx, strings.TrimSpace(record.ResultRef))
	if err != nil {
		return CreateTransferResult{}, err
	}
	if entry == nil {
		return CreateTransferResult{}, errs.Application(
			errs.CodeInvariantViolation,
			opCreateTransfer,
			"completed idempotency record points to missing journal",
			ErrReplayResultMissing,
		)
	}

	return CreateTransferResult{
		JournalID:      entry.ID(),
		Reference:      entry.Reference(),
		IdempotencyKey: key.String(),
		Replayed:       true,
		CreatedAt:      entry.CreatedAt(),
	}, nil
}

func buildTransferCreatedEvent(entry *domainledger.JournalEntry, cmd CreateTransferCommand) OutboxEvent {
	return OutboxEvent{
		Type:        "ledger.transfer.created",
		AggregateID: entry.ID(),
		OccurredAt:  entry.CreatedAt(),
		Payload: map[string]string{
			"journal_id":       entry.ID(),
			"reference":        entry.Reference(),
			"from_account_id":  cmd.FromAccountID,
			"to_account_id":    cmd.ToAccountID,
			"amount":           fmt.Sprintf("%d", cmd.Amount),
			"currency":         cmd.Currency,
			"idempotency_key":  cmd.IdempotencyKey,
			"transfer_command": cmd.ID,
		},
	}
}

func wrapIdempotencyDecisionError(decision idempotency.Decision, err error) error {
	if err == nil {
		return errs.Application(
			errs.CodeConflict,
			opCreateTransfer,
			"idempotency decision prevented transfer execution",
			fmt.Errorf("%w: %s", ErrInvalidCommand, decision),
		)
	}

	code := errs.CodeConflict
	if typed, ok := errs.As(err); ok {
		code = typed.Code
	}
	return errs.Application(
		code,
		opCreateTransfer,
		"idempotency decision prevented transfer execution",
		err,
	)
}

func unknownIdempotencyDecisionError() error {
	return errs.Application(
		errs.CodeInvariantViolation,
		opCreateTransfer,
		"unknown idempotency decision",
		idempotency.ErrUnknownStatus,
	)
}

func reservationRaceError(reference string) error {
	return errs.Application(
		errs.CodeConflict,
		opCreateTransfer,
		"idempotency reservation lost to a concurrent writer",
		fmt.Errorf("%w: %s", ErrReservationRace, strings.TrimSpace(reference)),
	)
}

func missingDependency(name string) error {
	return errs.Application(
		errs.CodeInternal,
		opNewTransferService,
		"missing transfer service dependency",
		fmt.Errorf("%w: %s", ErrMissingDependency, name),
	)
}

func invalidCommand(message string, err error) error {
	return errs.Application(
		errs.CodeInvalidArgument,
		opCreateTransfer,
		message,
		err,
	)
}
