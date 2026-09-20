package device

import (
	"context"
	"errors"
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

	return NewRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

// TestUpsertDevicePublicKey_FreesOrphanedDeviceFromPreviousOwner is the fix
// for a physical device legitimately changing hands between accounts:
// device_id is globally unique (uq_wallet_user_devices_device_id), separate
// from the (user_id, device_id) pair the ON CONFLICT clause targets. An
// inactive row under a different user means that owner already released the
// device (see DeactivateDevice), so it's cleared automatically rather than
// letting the insert fail on the device-alone constraint.
func TestUpsertDevicePublicKey_FreesOrphanedDeviceFromPreviousOwner(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "wallet_user_devices" WHERE`).
		WithArgs("device-1", "user-2", false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "wallet_user_devices"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.UpsertDevicePublicKey(context.Background(), &UserDevice{
		ID:         "row-1",
		UserID:     "user-2",
		DeviceID:   "device-1",
		PublicKey:  "pubkey",
		LastUsedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("UpsertDevicePublicKey: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestUpsertDevicePublicKey_ActiveDeviceOnDifferentUser_NotReassigned is the
// security boundary: a device_id that's still active for a different, live
// user must not be silently torn down and handed to whoever else claims that
// device_id in a request - the delete is scoped to is_active=false, so it
// no-ops here and the insert's own conflict error (device-alone constraint)
// propagates instead of a silent takeover.
func TestUpsertDevicePublicKey_ActiveDeviceOnDifferentUser_NotReassigned(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "wallet_user_devices" WHERE`).
		WithArgs("device-1", "user-2", false).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO "wallet_user_devices"`).
		WillReturnError(errors.New(`ERROR: duplicate key value violates unique constraint "uq_wallet_user_devices_device_id" (SQLSTATE 23505)`))
	mock.ExpectRollback()

	err := repo.UpsertDevicePublicKey(context.Background(), &UserDevice{
		ID:         "row-1",
		UserID:     "user-2",
		DeviceID:   "device-1",
		PublicKey:  "pubkey",
		LastUsedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestUpsertDevicePublicKey_SameOwner_StillUpserts confirms the common case
// (same user re-registering the same device, e.g. a public key rotation)
// still goes through the delete-then-insert transaction unchanged - the
// delete is a harmless no-op since "user_id <> ?" excludes the caller's own
// row.
func TestUpsertDevicePublicKey_SameOwner_StillUpserts(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "wallet_user_devices" WHERE`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO "wallet_user_devices"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.UpsertDevicePublicKey(context.Background(), &UserDevice{
		ID:         "row-1",
		UserID:     "user-1",
		DeviceID:   "device-1",
		PublicKey:  "rotated-pubkey",
		LastUsedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("UpsertDevicePublicKey: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
