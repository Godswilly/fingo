package ledger

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Godswilly/fingo/internal/domain/account"
	"github.com/Godswilly/fingo/internal/domain/idempotency"
	domainledger "github.com/Godswilly/fingo/internal/domain/ledger"
	"github.com/Godswilly/fingo/internal/errs"
)

type contextKey string

func TestNewTransferService_MissingDependency(t *testing.T) {
	t.Parallel()

	_, err := NewTransferService(TransferServiceDeps{})
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
	if !errors.Is(err, ErrMissingDependency) {
		t.Fatalf("expected missing dependency sentinel, got %v", err)
	}
	if !errs.IsLayer(err, errs.LayerApplication) {
		t.Fatalf("expected application-layer error, got %v", err)
	}
	if !errs.IsCode(err, errs.CodeInternal) {
		t.Fatalf("expected internal code, got %v", err)
	}
}

func TestTransferService_CreateTransfer_ReserveWinsExecutes(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	from := &account.Account{ID: cmd.FromAccountID, Owner: "source", Balance: 100, IsActive: true}
	to := &account.Account{ID: cmd.ToAccountID, Owner: "destination", Balance: 0, IsActive: true}
	service, deps := newTestTransferService(t, map[string]*account.Account{
		from.ID: from,
		to.ID:   to,
	})

	result, err := service.CreateTransfer(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	assertFreshTransferResult(t, result, cmd)
	if deps.idempotencies.reserveCalls != 1 {
		t.Fatalf("expected one reserve call, got %d", deps.idempotencies.reserveCalls)
	}
	if deps.idempotencies.reserveFingerprint != mustTransferFingerprint(t, cmd) {
		t.Fatalf("unexpected reserve fingerprint: %q", deps.idempotencies.reserveFingerprint)
	}
	if deps.idempotencies.markCalls != 0 {
		t.Fatalf("fresh reservation should not call MarkInProgress")
	}
	if deps.idempotencies.completedIdempotency != idempotency.Key(cmd.IdempotencyKey) {
		t.Fatalf("expected completed idempotency key %q, got %q", cmd.IdempotencyKey, deps.idempotencies.completedIdempotency)
	}
	if deps.idempotencies.completedResultRef != cmd.ID {
		t.Fatalf("expected completed result ref %q, got %q", cmd.ID, deps.idempotencies.completedResultRef)
	}
	if len(deps.journals.saved) != 1 {
		t.Fatalf("expected one journal save, got %d", len(deps.journals.saved))
	}
	if len(deps.outbox.saved) != 1 {
		t.Fatalf("expected one outbox event, got %d", len(deps.outbox.saved))
	}

	entry := deps.journals.saved[0]
	postings := entry.Postings()
	if len(postings) != 2 {
		t.Fatalf("expected two postings, got %d", len(postings))
	}
	assertPosting(t, postings[0], cmd.FromAccountID, cmd.Amount, domainledger.SideDebit, cmd.Currency)
	assertPosting(t, postings[1], cmd.ToAccountID, cmd.Amount, domainledger.SideCredit, cmd.Currency)
	assertOutboxEvent(t, deps.outbox.saved[0], cmd, entry)

	if from.Balance != 100 || to.Balance != 0 {
		t.Fatalf("transfer service should validate account balances without persisting account mutations")
	}
}

func TestTransferService_CreateTransfer_LocksAccountsInStableOrder(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	cmd.FromAccountID = "z-source"
	cmd.ToAccountID = "a-destination"
	service, deps := newTestTransferService(t, map[string]*account.Account{
		cmd.FromAccountID: {ID: cmd.FromAccountID, Owner: "source", Balance: 100, IsActive: true},
		cmd.ToAccountID:   {ID: cmd.ToAccountID, Owner: "destination", Balance: 0, IsActive: true},
	})

	_, err := service.CreateTransfer(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	want := []string{cmd.ToAccountID, cmd.FromAccountID}
	if !reflect.DeepEqual(deps.accounts.lockOrder, want) {
		t.Fatalf("unexpected lock order: want %v, got %v", want, deps.accounts.lockOrder)
	}
}

func TestTransferService_CreateTransfer_ReserveLosesReplaysFromJournal(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	replayEntry := mustJournalEntry(t, "jrn-existing", cmd.Reference, cmd.CreatedAt.Add(-1*time.Hour))
	service, deps := newTestTransferService(t, nil)
	deps.idempotencies.reserveReserved = false
	deps.idempotencies.reserveExisting = completedRecord(t, cmd, replayEntry.ID())
	deps.journals.byID[replayEntry.ID()] = replayEntry

	result, err := service.CreateTransfer(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected replay success, got %v", err)
	}

	assertReplayResult(t, result, cmd.IdempotencyKey, replayEntry)
	if len(deps.accounts.lockOrder) != 0 {
		t.Fatalf("replay should not lock accounts, got %v", deps.accounts.lockOrder)
	}
	if len(deps.journals.saved) != 0 {
		t.Fatalf("replay should not save journal entries")
	}
	if len(deps.outbox.saved) != 0 {
		t.Fatalf("replay should not save outbox events")
	}
	if deps.idempotencies.completedResultRef != "" {
		t.Fatalf("replay should not mutate completed idempotency state")
	}
}

func TestTransferService_CreateTransfer_ReserveLosesConflict(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	service, deps := newTestTransferService(t, nil)
	deps.idempotencies.reserveReserved = false
	deps.idempotencies.reserveExisting = &idempotency.Record{
		Key:         idempotency.Key(cmd.IdempotencyKey),
		Fingerprint: "different-fingerprint",
		Status:      idempotency.StatusCompleted,
		ResultRef:   "jrn-existing",
	}

	_, err := service.CreateTransfer(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !errors.Is(err, idempotency.ErrKeyConflict) {
		t.Fatalf("expected key conflict sentinel, got %v", err)
	}
	if !errs.IsCode(err, errs.CodeConflict) {
		t.Fatalf("expected conflict code, got %v", err)
	}
	assertNoTransferSideEffects(t, deps)
}

func TestTransferService_CreateTransfer_ReserveLosesRetryLater(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	service, deps := newTestTransferService(t, nil)
	deps.idempotencies.reserveReserved = false
	deps.idempotencies.reserveExisting = &idempotency.Record{
		Key:         idempotency.Key(cmd.IdempotencyKey),
		Fingerprint: mustTransferFingerprint(t, cmd),
		Status:      idempotency.StatusInProgress,
	}

	_, err := service.CreateTransfer(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected retry-later error")
	}
	if !errors.Is(err, idempotency.ErrRequestInProgress) {
		t.Fatalf("expected request-in-progress sentinel, got %v", err)
	}
	if !errs.IsCode(err, errs.CodeConflict) {
		t.Fatalf("expected conflict code, got %v", err)
	}
	assertNoTransferSideEffects(t, deps)
}

func TestTransferService_CreateTransfer_FailedRecordCASWinsExecutes(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	service, deps := newTestTransferService(t, map[string]*account.Account{
		cmd.FromAccountID: {ID: cmd.FromAccountID, Owner: "source", Balance: 100, IsActive: true},
		cmd.ToAccountID:   {ID: cmd.ToAccountID, Owner: "destination", Balance: 0, IsActive: true},
	})
	deps.idempotencies.reserveReserved = false
	deps.idempotencies.reserveExisting = failedRecord(t, cmd)
	deps.idempotencies.markOK = true

	result, err := service.CreateTransfer(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected failed record retry to execute, got %v", err)
	}

	assertFreshTransferResult(t, result, cmd)
	if deps.idempotencies.markCalls != 1 {
		t.Fatalf("expected one MarkInProgress call, got %d", deps.idempotencies.markCalls)
	}
	if deps.idempotencies.markFingerprint != mustTransferFingerprint(t, cmd) {
		t.Fatalf("unexpected mark fingerprint: %q", deps.idempotencies.markFingerprint)
	}
	if deps.idempotencies.completedIdempotency != idempotency.Key(cmd.IdempotencyKey) {
		t.Fatalf("expected completed idempotency key %q, got %q", cmd.IdempotencyKey, deps.idempotencies.completedIdempotency)
	}
	if deps.idempotencies.completedResultRef != cmd.ID {
		t.Fatalf("expected completed result ref %q, got %q", cmd.ID, deps.idempotencies.completedResultRef)
	}
	if len(deps.journals.saved) != 1 {
		t.Fatalf("expected one journal save, got %d", len(deps.journals.saved))
	}
	if len(deps.outbox.saved) != 1 {
		t.Fatalf("expected one outbox event, got %d", len(deps.outbox.saved))
	}
}

func TestTransferService_CreateTransfer_FailedRecordCASLosesCurrentInProgress(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	service, deps := newTestTransferService(t, nil)
	deps.idempotencies.reserveReserved = false
	deps.idempotencies.reserveExisting = failedRecord(t, cmd)
	deps.idempotencies.markOK = false
	deps.idempotencies.markCurrent = &idempotency.Record{
		Key:         idempotency.Key(cmd.IdempotencyKey),
		Fingerprint: mustTransferFingerprint(t, cmd),
		Status:      idempotency.StatusInProgress,
	}

	_, err := service.CreateTransfer(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected retry-later error")
	}
	if !errors.Is(err, idempotency.ErrRequestInProgress) {
		t.Fatalf("expected request-in-progress sentinel, got %v", err)
	}
	if !errs.IsCode(err, errs.CodeConflict) {
		t.Fatalf("expected conflict code, got %v", err)
	}
	assertNoTransferSideEffects(t, deps)
}

func TestTransferService_CreateTransfer_FailedRecordCASLosesCurrentCompleted(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	replayEntry := mustJournalEntry(t, "jrn-race-winner", cmd.Reference, cmd.CreatedAt.Add(-2*time.Minute))
	service, deps := newTestTransferService(t, nil)
	deps.idempotencies.reserveReserved = false
	deps.idempotencies.reserveExisting = failedRecord(t, cmd)
	deps.idempotencies.markOK = false
	deps.idempotencies.markCurrent = completedRecord(t, cmd, replayEntry.ID())
	deps.journals.byID[replayEntry.ID()] = replayEntry

	result, err := service.CreateTransfer(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected replay after CAS loss, got %v", err)
	}

	assertReplayResult(t, result, cmd.IdempotencyKey, replayEntry)
	assertNoTransferSideEffects(t, deps)
}

func TestTransferService_CreateTransfer_FailedRecordCASLosesCurrentStillFailed(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	service, deps := newTestTransferService(t, nil)
	deps.idempotencies.reserveReserved = false
	deps.idempotencies.reserveExisting = failedRecord(t, cmd)
	deps.idempotencies.markOK = false
	deps.idempotencies.markCurrent = failedRecord(t, cmd)

	_, err := service.CreateTransfer(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected reservation race error")
	}
	if !errors.Is(err, ErrReservationRace) {
		t.Fatalf("expected reservation race sentinel, got %v", err)
	}
	if !errs.IsCode(err, errs.CodeConflict) {
		t.Fatalf("expected conflict code, got %v", err)
	}
	assertNoTransferSideEffects(t, deps)
}

func TestTransferService_CreateTransfer_ReplayMissingJournalIsInvariantViolation(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	service, deps := newTestTransferService(t, nil)
	deps.idempotencies.reserveReserved = false
	deps.idempotencies.reserveExisting = completedRecord(t, cmd, "jrn-missing")

	_, err := service.CreateTransfer(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected replay invariant error")
	}
	if !errors.Is(err, ErrReplayResultMissing) {
		t.Fatalf("expected replay result missing sentinel, got %v", err)
	}
	if !errs.IsCode(err, errs.CodeInvariantViolation) {
		t.Fatalf("expected invariant violation code, got %v", err)
	}
	assertNoTransferSideEffects(t, deps)
}

func TestTransferService_CreateTransfer_InsufficientFunds(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	cmd.Amount = 150
	service, deps := newTestTransferService(t, map[string]*account.Account{
		cmd.FromAccountID: {ID: cmd.FromAccountID, Owner: "source", Balance: 100, IsActive: true},
		cmd.ToAccountID:   {ID: cmd.ToAccountID, Owner: "destination", Balance: 0, IsActive: true},
	})

	_, err := service.CreateTransfer(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected insufficient funds error")
	}
	if !errors.Is(err, account.ErrInsufficientFunds) {
		t.Fatalf("expected insufficient funds sentinel, got %v", err)
	}
	if len(deps.journals.saved) != 0 {
		t.Fatalf("insufficient funds should not save a journal")
	}
	if len(deps.outbox.saved) != 0 {
		t.Fatalf("insufficient funds should not save an outbox event")
	}
	if deps.idempotencies.completedResultRef != "" {
		t.Fatalf("insufficient funds should not mark idempotency completed")
	}
}

func TestTransferService_CreateTransfer_RejectsSameAccount(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	cmd.ToAccountID = cmd.FromAccountID
	service, deps := newTestTransferService(t, nil)

	_, err := service.CreateTransfer(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected same-account error")
	}
	if !errors.Is(err, ErrSameAccount) {
		t.Fatalf("expected same account sentinel, got %v", err)
	}
	if deps.tx.calls != 0 {
		t.Fatalf("same-account command should fail before opening transaction")
	}
}

func TestTransferService_CreateTransfer_PropagatesContext(t *testing.T) {
	t.Parallel()

	cmd := validTransferCommand()
	service, deps := newTestTransferService(t, map[string]*account.Account{
		cmd.FromAccountID: {ID: cmd.FromAccountID, Owner: "source", Balance: 100, IsActive: true},
		cmd.ToAccountID:   {ID: cmd.ToAccountID, Owner: "destination", Balance: 0, IsActive: true},
	})

	ctxKey := contextKey("request_id")
	ctx := context.WithValue(context.Background(), ctxKey, "req-123")
	_, err := service.CreateTransfer(ctx, cmd)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	assertContextValues(t, "idempotency store", deps.idempotencies.ctxValues, "req-123")
	assertContextValues(t, "account store", deps.accounts.ctxValues, "req-123")
	assertContextValues(t, "journal store", deps.journals.ctxValues, "req-123")
	assertContextValues(t, "outbox store", deps.outbox.ctxValues, "req-123")
}

type testDeps struct {
	tx            *fakeTransactor
	accounts      *fakeAccountStore
	journals      *fakeJournalStore
	idempotencies *fakeIdempotencyStore
	outbox        *fakeOutboxStore
}

func newTestTransferService(t *testing.T, accounts map[string]*account.Account) (*TransferService, testDeps) {
	t.Helper()

	deps := testDeps{
		tx:            &fakeTransactor{},
		accounts:      &fakeAccountStore{accounts: accounts},
		journals:      &fakeJournalStore{byID: make(map[string]*domainledger.JournalEntry)},
		idempotencies: &fakeIdempotencyStore{reserveReserved: true},
		outbox:        &fakeOutboxStore{},
	}
	service, err := NewTransferService(TransferServiceDeps{
		Transactor:    deps.tx,
		Accounts:      deps.accounts,
		Journals:      deps.journals,
		Idempotencies: deps.idempotencies,
		Outbox:        deps.outbox,
	})
	if err != nil {
		t.Fatalf("new transfer service: %v", err)
	}
	return service, deps
}

func validTransferCommand() CreateTransferCommand {
	return CreateTransferCommand{
		ID:             "jrn-001",
		IdempotencyKey: "idem-001",
		Reference:      "ref-001",
		Description:    "customer transfer",
		FromAccountID:  "acc-source",
		ToAccountID:    "acc-destination",
		Currency:       "USD",
		Amount:         40,
		CreatedAt:      time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		Metadata: map[string]string{
			"request_id": "req-001",
		},
	}
}

func mustTransferFingerprint(t *testing.T, cmd CreateTransferCommand) string {
	t.Helper()

	fp, err := idempotency.BuildFingerprint(idempotency.Command{
		Operation:     transferOperation,
		FromAccountID: cmd.FromAccountID,
		ToAccountID:   cmd.ToAccountID,
		Currency:      cmd.Currency,
		Reference:     cmd.Reference,
		Amount:        cmd.Amount,
	})
	if err != nil {
		t.Fatalf("build fingerprint: %v", err)
	}
	return fp
}

func completedRecord(t *testing.T, cmd CreateTransferCommand, resultRef string) *idempotency.Record {
	t.Helper()

	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	return &idempotency.Record{
		Key:         idempotency.Key(cmd.IdempotencyKey),
		Fingerprint: mustTransferFingerprint(t, cmd),
		Status:      idempotency.StatusCompleted,
		ResultRef:   resultRef,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func failedRecord(t *testing.T, cmd CreateTransferCommand) *idempotency.Record {
	t.Helper()

	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	return &idempotency.Record{
		Key:         idempotency.Key(cmd.IdempotencyKey),
		Fingerprint: mustTransferFingerprint(t, cmd),
		Status:      idempotency.StatusFailed,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func mustJournalEntry(t *testing.T, id, reference string, createdAt time.Time) *domainledger.JournalEntry {
	t.Helper()

	debit, err := domainledger.NewPosting("acc-source", 40, domainledger.SideDebit, "USD")
	if err != nil {
		t.Fatalf("setup debit posting: %v", err)
	}
	credit, err := domainledger.NewPosting("acc-destination", 40, domainledger.SideCredit, "USD")
	if err != nil {
		t.Fatalf("setup credit posting: %v", err)
	}
	entry, err := domainledger.NewJournalEntry(domainledger.CreateJournalEntryInput{
		ID:        id,
		Reference: reference,
		CreatedAt: createdAt,
		Postings:  []domainledger.Posting{debit, credit},
	})
	if err != nil {
		t.Fatalf("setup journal entry: %v", err)
	}
	return entry
}

func assertFreshTransferResult(t *testing.T, got CreateTransferResult, cmd CreateTransferCommand) {
	t.Helper()

	if got.JournalID != cmd.ID {
		t.Fatalf("unexpected journal id: want %q, got %q", cmd.ID, got.JournalID)
	}
	if got.Reference != cmd.Reference {
		t.Fatalf("unexpected reference: want %q, got %q", cmd.Reference, got.Reference)
	}
	if got.IdempotencyKey != cmd.IdempotencyKey {
		t.Fatalf("unexpected idempotency key: want %q, got %q", cmd.IdempotencyKey, got.IdempotencyKey)
	}
	if got.Replayed {
		t.Fatal("fresh transfer should not be marked replayed")
	}
	if !got.CreatedAt.Equal(cmd.CreatedAt) {
		t.Fatalf("unexpected created_at: want %v, got %v", cmd.CreatedAt, got.CreatedAt)
	}
}

func assertReplayResult(t *testing.T, got CreateTransferResult, wantKey string, entry *domainledger.JournalEntry) {
	t.Helper()

	if !got.Replayed {
		t.Fatal("expected result to be marked replayed")
	}
	if got.JournalID != entry.ID() {
		t.Fatalf("unexpected replay journal id: want %q, got %q", entry.ID(), got.JournalID)
	}
	if got.Reference != entry.Reference() {
		t.Fatalf("unexpected replay reference: want %q, got %q", entry.Reference(), got.Reference)
	}
	if got.IdempotencyKey != wantKey {
		t.Fatalf("unexpected replay idempotency key: want %q, got %q", wantKey, got.IdempotencyKey)
	}
	if !got.CreatedAt.Equal(entry.CreatedAt()) {
		t.Fatalf("unexpected replay created_at: want %v, got %v", entry.CreatedAt(), got.CreatedAt)
	}
}

func assertPosting(t *testing.T, got domainledger.Posting, wantAccountID string, wantAmount int64, wantSide domainledger.PostingSide, wantCurrency string) {
	t.Helper()

	if got.AccountID() != wantAccountID {
		t.Fatalf("unexpected posting account: want %q, got %q", wantAccountID, got.AccountID())
	}
	if got.Amount() != wantAmount {
		t.Fatalf("unexpected posting amount: want %d, got %d", wantAmount, got.Amount())
	}
	if got.Side() != wantSide {
		t.Fatalf("unexpected posting side: want %q, got %q", wantSide, got.Side())
	}
	if got.Currency() != wantCurrency {
		t.Fatalf("unexpected posting currency: want %q, got %q", wantCurrency, got.Currency())
	}
}

func assertOutboxEvent(t *testing.T, got OutboxEvent, cmd CreateTransferCommand, entry *domainledger.JournalEntry) {
	t.Helper()

	if got.Type != "ledger.transfer.created" {
		t.Fatalf("unexpected event type: %q", got.Type)
	}
	if got.AggregateID != entry.ID() {
		t.Fatalf("unexpected aggregate id: want %q, got %q", entry.ID(), got.AggregateID)
	}
	if !got.OccurredAt.Equal(entry.CreatedAt()) {
		t.Fatalf("unexpected occurred_at: want %v, got %v", entry.CreatedAt(), got.OccurredAt)
	}
	if got.Payload["from_account_id"] != cmd.FromAccountID {
		t.Fatalf("unexpected from account payload: %q", got.Payload["from_account_id"])
	}
	if got.Payload["to_account_id"] != cmd.ToAccountID {
		t.Fatalf("unexpected to account payload: %q", got.Payload["to_account_id"])
	}
	if got.Payload["idempotency_key"] != cmd.IdempotencyKey {
		t.Fatalf("unexpected idempotency key payload: %q", got.Payload["idempotency_key"])
	}
}

func assertNoTransferSideEffects(t *testing.T, deps testDeps) {
	t.Helper()

	if len(deps.accounts.lockOrder) != 0 {
		t.Fatalf("expected no account locks, got %v", deps.accounts.lockOrder)
	}
	if len(deps.journals.saved) != 0 {
		t.Fatalf("expected no journal saves, got %d", len(deps.journals.saved))
	}
	if len(deps.outbox.saved) != 0 {
		t.Fatalf("expected no outbox saves, got %d", len(deps.outbox.saved))
	}
	if deps.idempotencies.completedResultRef != "" {
		t.Fatalf("expected no idempotency completion, got %q", deps.idempotencies.completedResultRef)
	}
}

func assertContextValues(t *testing.T, name string, got []any, want any) {
	t.Helper()

	if len(got) == 0 {
		t.Fatalf("%s did not receive context", name)
	}
	for _, value := range got {
		if value != want {
			t.Fatalf("%s did not receive context value %v: got %v", name, want, value)
		}
	}
}

type fakeTransactor struct {
	calls int
}

func (f *fakeTransactor) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

type fakeAccountStore struct {
	accounts  map[string]*account.Account
	lockOrder []string
	ctxValues []any
}

func (f *fakeAccountStore) GetForUpdate(ctx context.Context, id string) (*account.Account, error) {
	f.lockOrder = append(f.lockOrder, id)
	f.ctxValues = append(f.ctxValues, ctx.Value(contextKey("request_id")))
	return f.accounts[id], nil
}

type fakeJournalStore struct {
	byID      map[string]*domainledger.JournalEntry
	saved     []*domainledger.JournalEntry
	ctxValues []any
}

func (f *fakeJournalStore) Get(ctx context.Context, id string) (*domainledger.JournalEntry, error) {
	f.ctxValues = append(f.ctxValues, ctx.Value(contextKey("request_id")))
	return f.byID[id], nil
}

func (f *fakeJournalStore) Save(ctx context.Context, entry *domainledger.JournalEntry) error {
	f.saved = append(f.saved, entry)
	f.byID[entry.ID()] = entry
	f.ctxValues = append(f.ctxValues, ctx.Value(contextKey("request_id")))
	return nil
}

type fakeIdempotencyStore struct {
	reserveExisting      *idempotency.Record
	reserveReserved      bool
	reserveCalls         int
	markOK               bool
	markCurrent          *idempotency.Record
	markCalls            int
	completedResultRef   string
	ctxValues            []any
	reserveFingerprint   string
	markFingerprint      string
	completedIdempotency idempotency.Key
}

func (f *fakeIdempotencyStore) Reserve(ctx context.Context, key idempotency.Key, fingerprint string) (*idempotency.Record, bool, error) {
	f.reserveCalls++
	f.reserveFingerprint = fingerprint
	f.ctxValues = append(f.ctxValues, ctx.Value(contextKey("request_id")))
	return f.reserveExisting, f.reserveReserved, nil
}

func (f *fakeIdempotencyStore) MarkInProgress(ctx context.Context, key idempotency.Key, fingerprint string) (bool, *idempotency.Record, error) {
	f.markCalls++
	f.markFingerprint = fingerprint
	f.ctxValues = append(f.ctxValues, ctx.Value(contextKey("request_id")))
	return f.markOK, f.markCurrent, nil
}

func (f *fakeIdempotencyStore) SaveCompleted(ctx context.Context, key idempotency.Key, resultRef string) error {
	f.ctxValues = append(f.ctxValues, ctx.Value(contextKey("request_id")))
	f.completedIdempotency = key
	f.completedResultRef = resultRef
	return nil
}

type fakeOutboxStore struct {
	saved     []OutboxEvent
	ctxValues []any
}

func (f *fakeOutboxStore) Save(ctx context.Context, event OutboxEvent) error {
	f.saved = append(f.saved, event)
	f.ctxValues = append(f.ctxValues, ctx.Value(contextKey("request_id")))
	return nil
}
