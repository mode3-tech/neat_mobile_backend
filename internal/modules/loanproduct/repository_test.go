package loanproduct

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockRepository(t *testing.T) (*Repository, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		DisableAutomaticPing: true,
	})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("open gorm db: %v", err)
	}

	cleanup := func() {
		_ = sqlDB.Close()
	}

	return NewRepository(gormDB), mock, cleanup
}

func getUserQueryPattern() string {
	return regexp.QuoteMeta(`SELECT core_customer_id, phone, dob, bvn, nin, is_phone_verified, is_bvn_verified, is_nin_verified, pin_hash, failed_transaction_pin_attempts, transaction_pin_locked_until FROM "wallet_users" WHERE id = $1 LIMIT $2`)
}

func updateUserCoreCustomerIDQueryPattern() string {
	return regexp.QuoteMeta(`UPDATE "wallet_users" SET "core_customer_id"=$1 WHERE id = $2`)
}

func updateTransactionPinAttemptsQueryPattern() string {
	return regexp.QuoteMeta(`UPDATE "wallet_users" SET "failed_transaction_pin_attempts"=$1,"transaction_pin_locked_until"=$2 WHERE id = $3`)
}

func TestRepository_GetUser_ReturnsCoreCustomerID(t *testing.T) {
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
	}).AddRow("2048", "08012345678", dob, "12345678901", "12345678901", true, true, true, "hashed-pin", 2, nil)

	mock.ExpectQuery(getUserQueryPattern()).
		WithArgs("user-1", 1).
		WillReturnRows(rows)

	user, err := repo.GetUser(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetUser returned error: %v", err)
	}
	if user == nil {
		t.Fatal("GetUser returned nil user")
	}
	if user.CoreCustomerID == nil || *user.CoreCustomerID != "2048" {
		t.Fatalf("unexpected core customer id: %#v", user.CoreCustomerID)
	}
	if user.PinHash != "hashed-pin" {
		t.Fatalf("unexpected pin hash: %q", user.PinHash)
	}
	if user.FailedTransactionAttempts != 2 {
		t.Fatalf("unexpected failed transaction attempts: %d", user.FailedTransactionAttempts)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRepository_UpdateUserCoreCustomerID_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(updateUserCoreCustomerIDQueryPattern()).
		WithArgs("2048", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateUserCoreCustomerID(context.Background(), "user-1", "2048"); err != nil {
		t.Fatalf("UpdateUserCoreCustomerID returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRepository_UpdateUserCoreCustomerID_MissingUser(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(updateUserCoreCustomerIDQueryPattern()).
		WithArgs("2048", "missing-user").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.UpdateUserCoreCustomerID(context.Background(), "missing-user", "2048")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRepository_UpdateTransactionPinAttempts_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	lockedUntil := time.Date(2026, 3, 20, 12, 15, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec(updateTransactionPinAttemptsQueryPattern()).
		WithArgs(5, lockedUntil, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateTransactionPinAttempts(context.Background(), "user-1", 5, &lockedUntil); err != nil {
		t.Fatalf("UpdateTransactionPinAttempts returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRepository_ResetTransactionPinAttempts_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(updateTransactionPinAttemptsQueryPattern()).
		WithArgs(0, nil, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.ResetTransactionPinAttempts(context.Background(), "user-1"); err != nil {
		t.Fatalf("ResetTransactionPinAttempts returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
