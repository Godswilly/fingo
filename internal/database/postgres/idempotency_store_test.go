//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Godswilly/fingo/internal/database/postgres"
	"github.com/Godswilly/fingo/internal/domain/idempotency"
)

func TestIdempotencyStore_Reserve_FreshKey(t *testing.T) {
	truncateAll(t)

	store := postgres.NewIdempotencyStore(testDB)
	key, err := idempotency.NewKey("key-1")
	if err != nil {
		t.Fatal(err)
	}

	var existing *idempotency.Record
	var reserved bool
	withinTx(t, func(ctx context.Context) error {
		var err error
		existing, reserved, err = store.Reserve(ctx, key, "fp-1")
		return err
	})
	if !reserved {
		t.Fatal("expected reserved=true for a fresh key")
	}
	if existing != nil {
		t.Fatalf("expected nil existing record, got %+v", existing)
	}
}

func TestIdempotencyStore_Reserve_ExistingKeyIsLockedNotReserved(t *testing.T) {
	truncateAll(t)

	store := postgres.NewIdempotencyStore(testDB)
	key, err := idempotency.NewKey("key-2")
	if err != nil {
		t.Fatal(err)
	}
	var existing *idempotency.Record
	var reserved bool
	withinTx(t, func(txCtx context.Context) error {
		var err error
		_, reserved, err = store.Reserve(txCtx, key, "fp-2")
		if err != nil || !reserved {
			t.Fatalf("first reserve: reserved=%v err=%v", reserved, err)
		}
		existing, reserved, err = store.Reserve(txCtx, key, "fp-2")
		return err
	})
	if reserved {
		t.Fatal("expected reserved=false once a row already exists")
	}
	if existing == nil || existing.Status != idempotency.StatusInProgress {
		t.Fatalf("expected locked in_progress record, got %+v", existing)
	}
}

func TestIdempotencyStore_MarkInProgress_CompareAndSwap(t *testing.T) {
	truncateAll(t)

	store := postgres.NewIdempotencyStore(testDB)
	key, err := idempotency.NewKey("key-3")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	withinTx(t, func(txCtx context.Context) error {
		_, _, err := store.Reserve(txCtx, key, "fp-3")
		return err
	})
	// Only an out-of-band reaper produces failed rows in production; force
	// one directly here to exercise the retry-claim CAS.
	if _, err := testDB.Pool.Exec(ctx, `UPDATE idempotency_keys SET status = 'failed' WHERE key = $1`, key.String()); err != nil {
		t.Fatalf("force failed status: %v", err)
	}

	var ok bool
	var current *idempotency.Record
	withinTx(t, func(txCtx context.Context) error {
		var err error
		ok, current, err = store.MarkInProgress(txCtx, key, "fp-3")
		return err
	})
	if !ok {
		t.Fatal("expected CAS to succeed against a failed row")
	}
	if current.Status != idempotency.StatusInProgress {
		t.Fatalf("expected in_progress status, got %s", current.Status)
	}

	var okAgain bool
	var currentAgain *idempotency.Record
	withinTx(t, func(txCtx context.Context) error {
		var err error
		okAgain, currentAgain, err = store.MarkInProgress(txCtx, key, "fp-3")
		return err
	})
	if okAgain {
		t.Fatal("expected CAS to fail: row is no longer failed")
	}
	if currentAgain.Status != idempotency.StatusInProgress {
		t.Fatalf("expected fresh locked row still in_progress, got %s", currentAgain.Status)
	}
}

func TestIdempotencyStore_SaveCompleted(t *testing.T) {
	truncateAll(t)

	store := postgres.NewIdempotencyStore(testDB)
	key, err := idempotency.NewKey("key-4")
	if err != nil {
		t.Fatal(err)
	}
	withinTx(t, func(txCtx context.Context) error {
		if _, _, err := store.Reserve(txCtx, key, "fp-4"); err != nil {
			return err
		}
		return store.SaveCompleted(txCtx, key, "journal-4")
	})

	var existing *idempotency.Record
	var reserved bool
	withinTx(t, func(txCtx context.Context) error {
		var err error
		existing, reserved, err = store.Reserve(txCtx, key, "fp-4")
		return err
	})
	if reserved {
		t.Fatal("expected reserved=false since the key already exists")
	}
	if existing.Status != idempotency.StatusCompleted || existing.ResultRef != "journal-4" {
		t.Fatalf("expected completed record with result ref, got %+v", existing)
	}
}

func TestIdempotencyStore_SaveCompleted_MissingKey(t *testing.T) {
	truncateAll(t)

	store := postgres.NewIdempotencyStore(testDB)
	key, err := idempotency.NewKey("missing-key")
	if err != nil {
		t.Fatal(err)
	}

	withinTx(t, func(ctx context.Context) error {
		if err := store.SaveCompleted(ctx, key, "journal-x"); err == nil {
			t.Fatal("expected error when completing a key that was never reserved")
		}
		return nil
	})
}

func TestIdempotencyStore_ReservationRace(t *testing.T) {
	truncateAll(t)

	store := postgres.NewIdempotencyStore(testDB)
	key, err := idempotency.NewKey("race-key")
	if err != nil {
		t.Fatal(err)
	}

	const attempts = 10
	var reservedCount int32
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := postgres.NewTransactor(testDB).WithinTx(context.Background(), func(ctx context.Context) error {
				_, reserved, err := store.Reserve(ctx, key, "fp-race")
				if err != nil {
					return err
				}
				if reserved {
					atomic.AddInt32(&reservedCount, 1)
				}
				return nil
			})
			if err != nil {
				t.Errorf("reserve: %v", err)
			}
		}()
	}
	wg.Wait()

	if reservedCount != 1 {
		t.Fatalf("expected exactly one concurrent Reserve to win, got %d", reservedCount)
	}
}
