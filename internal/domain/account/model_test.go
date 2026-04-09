package account

import (
	"errors"
	"testing"

	"github.com/Godswilly/fingo/internal/errs"
)

func TestNewAccount_InvalidOwner_ReturnsTypedDomainError(t *testing.T) {
	_, err := NewAccount("acc-1", "", 1000)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidOwner) {
		t.Fatalf("expected wrapped ErrInvalidOwner, got %v", err)
	}
	if !errs.IsLayer(err, errs.LayerDomain) {
		t.Fatalf("expected domain layer error, got %v", err)
	}
	if !errs.IsCode(err, errs.CodeInvalidArgument) {
		t.Fatalf("expected invalid argument code, got %v", err)
	}
}

func TestNewAccount_NegativeBalance_ReturnsTypedDomainError(t *testing.T) {
	_, err := NewAccount("acc-1", "alice", -1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInitialBalance) {
		t.Fatalf("expected wrapped ErrInitialBalance, got %v", err)
	}
	if !errs.IsLayer(err, errs.LayerDomain) {
		t.Fatalf("expected domain layer error, got %v", err)
	}
	if !errs.IsCode(err, errs.CodeInvalidArgument) {
		t.Fatalf("expected invalid argument code, got %v", err)
	}
}

func TestNewAccount_Valid(t *testing.T) {
	acc, err := NewAccount("acc-1", "alice", 1000)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if acc.ID != "acc-1" || acc.Owner != "alice" || acc.Balance != 1000 || !acc.IsActive {
		t.Fatalf("unexpected account state: %+v", acc)
	}
}
