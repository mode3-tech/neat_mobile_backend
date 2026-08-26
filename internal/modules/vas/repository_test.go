package vas

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockVASRepository(t *testing.T) (*Repository, sqlmock.Sqlmock, func()) {
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

func lockUserPattern() string {
	return regexp.QuoteMeta(`SELECT "id" FROM "wallet_users" WHERE id = $1 ORDER BY "wallet_users"."id" LIMIT $2 FOR UPDATE`)
}

func findReservationByTxPattern() string {
	return regexp.QuoteMeta(`SELECT * FROM "wallet_cashbacks" WHERE transaction_id = $1 AND entry_type = $2 ORDER BY "wallet_cashbacks"."id" LIMIT $3`)
}

func findReversalByTxPattern() string {
	return findReservationByTxPattern()
}

func latestCashbackForUserPattern() string {
	return regexp.QuoteMeta(`SELECT * FROM "wallet_cashbacks" WHERE mobile_user_id = $1 ORDER BY created_at DESC`)
}

func insertCashbackPattern() string {
	return regexp.QuoteMeta(`INSERT INTO "wallet_cashbacks"`)
}

func TestReserveCashbackSpend_CapsToAvailableBalance(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockUserPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("user-1"))
	mock.ExpectQuery(findReservationByTxPattern()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(latestCashbackForUserPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "cashback_before", "cashback_after", "cashback_amount", "cashback_source", "entry_type", "transaction_id", "created_at"}).
			AddRow("cb-0", "user-1", 0, 5000, 5000, "referral", "credit", nil, time.Now()))
	mock.ExpectExec(insertCashbackPattern()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "wallet_transactions" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	reserved, err := repo.ReserveCashbackSpend(context.Background(), "tx-1", "user-1", 10000, "vas")
	if err != nil {
		t.Fatalf("ReserveCashbackSpend returned error: %v", err)
	}
	if reserved != 5000 {
		t.Fatalf("reserved = %d, want 5000 (capped to available balance)", reserved)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestReserveCashbackSpend_ZeroBalance_NoOp(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockUserPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("user-1"))
	mock.ExpectQuery(findReservationByTxPattern()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(latestCashbackForUserPattern()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "wallet_transactions" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	reserved, err := repo.ReserveCashbackSpend(context.Background(), "tx-2", "user-1", 10000, "vas")
	if err != nil {
		t.Fatalf("ReserveCashbackSpend returned error: %v", err)
	}
	if reserved != 0 {
		t.Fatalf("reserved = %d, want 0 when the user has no cashback balance", reserved)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestReserveCashbackSpend_Idempotent_ReturnsExistingReservation(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockUserPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("user-1"))
	mock.ExpectQuery(findReservationByTxPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "cashback_before", "cashback_after", "cashback_amount", "cashback_source", "entry_type", "transaction_id", "created_at"}).
			AddRow("tx-3-cashback", "user-1", 5000, 2000, 3000, "vas", "debit", "tx-3", time.Now()))
	mock.ExpectCommit()

	reserved, err := repo.ReserveCashbackSpend(context.Background(), "tx-3", "user-1", 10000, "vas")
	if err != nil {
		t.Fatalf("ReserveCashbackSpend returned error: %v", err)
	}
	if reserved != 3000 {
		t.Fatalf("reserved = %d, want 3000 from the existing reservation (no second debit)", reserved)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestCompleteCashbackSpend_UsesExistingReservation_NoDoubleDebit(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","wallet_id" FROM "wallet_users" WHERE id = $1 ORDER BY "wallet_users"."id" LIMIT $2 FOR UPDATE`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "wallet_id"}).AddRow("user-1", "wallet-1"))
	mock.ExpectQuery(findReservationByTxPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "cashback_before", "cashback_after", "cashback_amount", "cashback_source", "entry_type", "transaction_id", "created_at"}).
			AddRow("tx-4-cashback", "user-1", 5000, 2000, 3000, "vas", "debit", "tx-4", time.Now()))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "wallet_transactions" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	after, err := repo.CompleteCashbackSpend(context.Background(), "tx-4", "user-1", 3000, "vas", TransactionStatusSuccessful, 100000)
	if err != nil {
		t.Fatalf("CompleteCashbackSpend returned error: %v", err)
	}
	if after != 2000 {
		t.Fatalf("after = %d, want 2000 (the reserved row's cashback_after, no new debit created)", after)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestReleaseCashbackSpend_CreditsBackReservedAmount(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockUserPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("user-1"))
	mock.ExpectQuery(findReservationByTxPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "cashback_before", "cashback_after", "cashback_amount", "cashback_source", "entry_type", "transaction_id", "created_at"}).
			AddRow("tx-5-cashback", "user-1", 5000, 2000, 3000, "vas", "debit", "tx-5", time.Now()))
	mock.ExpectQuery(findReversalByTxPattern()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(latestCashbackForUserPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "cashback_before", "cashback_after", "cashback_amount", "cashback_source", "entry_type", "transaction_id", "created_at"}).
			AddRow("tx-5-cashback", "user-1", 5000, 2000, 3000, "vas", "debit", "tx-5", time.Now()))
	mock.ExpectExec(insertCashbackPattern()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.ReleaseCashbackSpend(context.Background(), "tx-5", "user-1", "vas"); err != nil {
		t.Fatalf("ReleaseCashbackSpend returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestReleaseCashbackSpend_NoOpWhenNoReservationExists(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockUserPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("user-1"))
	mock.ExpectQuery(findReservationByTxPattern()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectCommit()

	if err := repo.ReleaseCashbackSpend(context.Background(), "tx-6", "user-1", "vas"); err != nil {
		t.Fatalf("ReleaseCashbackSpend returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestReleaseCashbackSpend_IdempotentWhenAlreadyReversed(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockUserPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("user-1"))
	mock.ExpectQuery(findReservationByTxPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "cashback_before", "cashback_after", "cashback_amount", "cashback_source", "entry_type", "transaction_id", "created_at"}).
			AddRow("tx-7-cashback", "user-1", 5000, 2000, 3000, "vas", "debit", "tx-7", time.Now()))
	mock.ExpectQuery(findReversalByTxPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "cashback_before", "cashback_after", "cashback_amount", "cashback_source", "entry_type", "transaction_id", "created_at"}).
			AddRow("tx-7-cashback-reversal", "user-1", 2000, 5000, 3000, "vas", "reversal", "tx-7", time.Now()))
	mock.ExpectCommit()

	if err := repo.ReleaseCashbackSpend(context.Background(), "tx-7", "user-1", "vas"); err != nil {
		t.Fatalf("ReleaseCashbackSpend returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
