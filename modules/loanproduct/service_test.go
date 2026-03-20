package loanproduct

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func hashTestPin(t *testing.T, pin string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash pin: %v", err)
	}

	return string(hash)
}

func TestServiceApplyForLoan_IncorrectTransactionPin(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	dob := time.Date(1995, 7, 10, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"core_customer_id",
		"phone",
		"dob",
		"bvn",
		"nin",
		"is_phone_verified",
		"is_bvn_verified",
		"is_nin_verified",
		"pin_hash",
		"failed_transaction_pin_attempts",
		"transaction_pin_locked_until",
	}).AddRow("2048", "08012345678", dob, "12345678901", "12345678901", true, true, true, hashTestPin(t, "1234"), 0, nil)

	mock.ExpectQuery(getUserQueryPattern()).
		WithArgs("user-1", 1).
		WillReturnRows(rows)

	mock.ExpectBegin()
	mock.ExpectExec(updateTransactionPinAttemptsQueryPattern()).
		WithArgs(1, nil, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	service := NewService(repo, nil, nil)

	_, err := service.ApplyForLoan(context.Background(), LoanRequest{TransactionPin: "0000"}, "user-1")
	if !errors.Is(err, ErrIncorrectTransactionPin) {
		t.Fatalf("expected ErrIncorrectTransactionPin, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestServiceApplyForLoan_LocksAfterFifthIncorrectTransactionPin(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	dob := time.Date(1995, 7, 10, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"core_customer_id",
		"phone",
		"dob",
		"bvn",
		"nin",
		"is_phone_verified",
		"is_bvn_verified",
		"is_nin_verified",
		"pin_hash",
		"failed_transaction_pin_attempts",
		"transaction_pin_locked_until",
	}).AddRow("2048", "08012345678", dob, "12345678901", "12345678901", true, true, true, hashTestPin(t, "1234"), 4, nil)

	mock.ExpectQuery(getUserQueryPattern()).
		WithArgs("user-1", 1).
		WillReturnRows(rows)

	mock.ExpectBegin()
	mock.ExpectExec(updateTransactionPinAttemptsQueryPattern()).
		WithArgs(5, sqlmock.AnyArg(), "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	service := NewService(repo, nil, nil)

	_, err := service.ApplyForLoan(context.Background(), LoanRequest{TransactionPin: "0000"}, "user-1")
	if !errors.Is(err, ErrTooManyTransactionPinAttempts) {
		t.Fatalf("expected ErrTooManyTransactionPinAttempts, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestServiceApplyForLoan_RejectsLockedTransactionPin(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	dob := time.Date(1995, 7, 10, 0, 0, 0, 0, time.UTC)
	lockedUntil := time.Now().Add(10 * time.Minute)
	rows := sqlmock.NewRows([]string{
		"core_customer_id",
		"phone",
		"dob",
		"bvn",
		"nin",
		"is_phone_verified",
		"is_bvn_verified",
		"is_nin_verified",
		"pin_hash",
		"failed_transaction_pin_attempts",
		"transaction_pin_locked_until",
	}).AddRow("2048", "08012345678", dob, "12345678901", "12345678901", true, true, true, hashTestPin(t, "1234"), 5, lockedUntil)

	mock.ExpectQuery(getUserQueryPattern()).
		WithArgs("user-1", 1).
		WillReturnRows(rows)

	service := NewService(repo, nil, nil)

	_, err := service.ApplyForLoan(context.Background(), LoanRequest{TransactionPin: "1234"}, "user-1")
	if !errors.Is(err, ErrTransactionPinTemporarilyLocked) {
		t.Fatalf("expected ErrTransactionPinTemporarilyLocked, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
