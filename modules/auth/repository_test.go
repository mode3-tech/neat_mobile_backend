package auth

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

	return NewRespository(gormDB), mock, cleanup
}

func getUserByEmailQueryPattern() string {
	return regexp.QuoteMeta(`SELECT id,email,password,created_at FROM "wallet_users" WHERE email = $1 ORDER BY "wallet_users"."id" LIMIT $2`)
}

func getBVNRecordByBVNQueryPattern() string {
	return regexp.QuoteMeta(`SELECT id, user_id FROM "wallet_bvn_records" WHERE bvn = $1 LIMIT $2`)
}

func updateBVNRecordUserIDQueryPattern() string {
	return regexp.QuoteMeta(`UPDATE "wallet_bvn_records" SET "user_id"=$1 WHERE id = $2`)
}

func TestRepository_GetUserByEmail_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	email := "user@example.com"
	createdAt := time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"id", "email", "password", "created_at"}).
		AddRow("user-1", email, "hashed-password", createdAt)

	mock.ExpectQuery(getUserByEmailQueryPattern()).
		WillReturnRows(rows)

	user, err := repo.GetUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("GetUserByEmail returned error: %v", err)
	}
	if user == nil {
		t.Fatal("GetUserByEmail returned nil user")
	}

	if user.ID != "user-1" {
		t.Fatalf("unexpected user ID: got %q", user.ID)
	}
	if user.Email != email {
		t.Fatalf("unexpected user email: got %q", user.Email)
	}
	if user.PasswordHash != "hashed-password" {
		t.Fatalf("unexpected password hash: got %q", user.PasswordHash)
	}
	if !user.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected created_at: got %v want %v", user.CreatedAt, createdAt)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRepository_GetUserByEmail_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectQuery(getUserByEmailQueryPattern()).
		WillReturnError(gorm.ErrRecordNotFound)

	user, err := repo.GetUserByEmail(context.Background(), "missing@example.com")
	if user != nil {
		t.Fatalf("expected nil user, got %+v", user)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRepository_LinkBVNRecordToUser_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "user_id"}).AddRow("bvn-row-1", "")

	mock.ExpectQuery(getBVNRecordByBVNQueryPattern()).
		WithArgs("12345678901", 1).
		WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(updateBVNRecordUserIDQueryPattern()).
		WithArgs("user-1", "bvn-row-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.LinkBVNRecordToUser(context.Background(), "12345678901", "user-1"); err != nil {
		t.Fatalf("LinkBVNRecordToUser returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRepository_LinkBVNRecordToUser_MissingRecordIsIgnored(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectQuery(getBVNRecordByBVNQueryPattern()).
		WithArgs("12345678901", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	if err := repo.LinkBVNRecordToUser(context.Background(), "12345678901", "user-1"); err != nil {
		t.Fatalf("LinkBVNRecordToUser returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRepository_LinkBVNRecordToUser_RejectsDifferentUser(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "user_id"}).AddRow("bvn-row-1", "other-user")

	mock.ExpectQuery(getBVNRecordByBVNQueryPattern()).
		WithArgs("12345678901", 1).
		WillReturnRows(rows)

	err := repo.LinkBVNRecordToUser(context.Background(), "12345678901", "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "bvn already linked to another user" {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
