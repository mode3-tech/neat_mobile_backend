package smsbilling

import (
	"context"
	"regexp"
	"testing"

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

func insertChargePattern() string {
	return regexp.QuoteMeta(`INSERT INTO "wallet_sms_charges"`)
}

func lockChargePattern() string {
	return `SELECT \* FROM "wallet_sms_charges" WHERE`
}

func lockWalletPattern() string {
	return `SELECT \* FROM "wallet_customer_wallets" WHERE`
}

func insertLedgerTxPattern() string {
	return regexp.QuoteMeta(`INSERT INTO "wallet_transactions"`)
}

func updateWalletBalancePattern() string {
	return regexp.QuoteMeta(`UPDATE "wallet_customer_wallets" SET`)
}

func updateChargePattern() string {
	return regexp.QuoteMeta(`UPDATE "wallet_sms_charges" SET`)
}

func chargeRows(id, mobileUserID, status string, amountKobo int64, attempts int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "mobile_user_id", "phone", "purpose", "units", "unit_price_kobo", "amount_kobo", "status", "reference", "attempts"}).
		AddRow(id, mobileUserID, "+2348000000000", "login", 1, amountKobo, amountKobo, status, "sms-charge-"+id, attempts)
}

func walletRows(internalWalletID, mobileUserID string, availableBalance, bookedBalance int64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"internal_wallet_id", "mobile_user_id", "available_balance", "booked_balance"}).
		AddRow(internalWalletID, mobileUserID, availableBalance, bookedBalance)
}

func TestCreateCharge_SnapshotsPriceAndComputesAmount(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(insertChargePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	charge, err := repo.CreateCharge(context.Background(), "user-1", "+2348000000000", "login", 2, 400)
	if err != nil {
		t.Fatalf("CreateCharge: %v", err)
	}
	if charge.AmountKobo != 800 {
		t.Fatalf("expected amount 800 (2 units x 400 kobo), got %d", charge.AmountKobo)
	}
	if charge.UnitPriceKobo != 400 {
		t.Fatalf("expected unit price snapshot 400, got %d", charge.UnitPriceKobo)
	}
	if charge.Status != SMSChargeStatusPending {
		t.Fatalf("expected pending status, got %s", charge.Status)
	}
	if charge.Reference == "" {
		t.Fatalf("expected a non-empty idempotency reference")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAttemptCollect_SuccessfulDebit(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockChargePattern()).
		WillReturnRows(chargeRows("charge-1", "user-1", string(SMSChargeStatusPending), 400, 0))
	mock.ExpectQuery(lockWalletPattern()).
		WillReturnRows(walletRows("wallet-1", "user-1", 10_000, 10_000))
	mock.ExpectExec(insertLedgerTxPattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(updateWalletBalancePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(updateChargePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.AttemptCollect(context.Background(), "charge-1", 100)
	if err != nil {
		t.Fatalf("AttemptCollect: %v", err)
	}
	if result != CollectResultCollected {
		t.Fatalf("expected collected, got %s", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAttemptCollect_InsufficientBalance_LeavesChargePending(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockChargePattern()).
		WillReturnRows(chargeRows("charge-1", "user-1", string(SMSChargeStatusPending), 400, 0))
	mock.ExpectQuery(lockWalletPattern()).
		WillReturnRows(walletRows("wallet-1", "user-1", 100, 100))
	// No ledger insert, no wallet update - only the charge's attempt counter moves.
	mock.ExpectExec(updateChargePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.AttemptCollect(context.Background(), "charge-1", 100)
	if err != nil {
		t.Fatalf("AttemptCollect: %v", err)
	}
	if result != CollectResultInsufficient {
		t.Fatalf("expected insufficient, got %s", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAttemptCollect_NoWalletYet_TreatedAsRetryable(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockChargePattern()).
		WillReturnRows(chargeRows("charge-1", "user-1", string(SMSChargeStatusPending), 400, 0))
	mock.ExpectQuery(lockWalletPattern()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectExec(updateChargePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.AttemptCollect(context.Background(), "charge-1", 100)
	if err != nil {
		t.Fatalf("AttemptCollect: %v", err)
	}
	if result != CollectResultInsufficient {
		t.Fatalf("expected insufficient (no wallet yet), got %s", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAttemptCollect_AbandonsAfterMaxAttempts(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	// Already at attempts=2; maxAttempts=3 means this attempt (making it 3)
	// crosses the threshold and the charge should be abandoned, not retried
	// forever.
	mock.ExpectBegin()
	mock.ExpectQuery(lockChargePattern()).
		WillReturnRows(chargeRows("charge-1", "user-1", string(SMSChargeStatusPending), 400, 2))
	mock.ExpectQuery(lockWalletPattern()).
		WillReturnRows(walletRows("wallet-1", "user-1", 0, 0))
	mock.ExpectExec(updateChargePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.AttemptCollect(context.Background(), "charge-1", 3)
	if err != nil {
		t.Fatalf("AttemptCollect: %v", err)
	}
	if result != CollectResultAbandoned {
		t.Fatalf("expected abandoned, got %s", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestAttemptCollect_AlreadyResolved_IsIdempotent is the mechanism that makes
// double-collection impossible: the lock query filters on status = pending,
// so a charge a concurrent attempt already collected (or a stale sweep entry
// that raced credit-triggered recovery) simply isn't found, and nothing else
// runs.
func TestAttemptCollect_AlreadyResolved_IsIdempotent(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(lockChargePattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "phone", "purpose", "units", "unit_price_kobo", "amount_kobo", "status", "reference", "attempts"}))
	mock.ExpectCommit()

	result, err := repo.AttemptCollect(context.Background(), "charge-1", 100)
	if err != nil {
		t.Fatalf("AttemptCollect: %v", err)
	}
	if result != CollectResultSkipped {
		t.Fatalf("expected skipped, got %s", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListOutstandingForUser_ReturnsPendingIDs(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT "id" FROM "wallet_sms_charges" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("charge-1").AddRow("charge-2"))

	ids, err := repo.ListOutstandingForUser(context.Background(), "user-1", 20)
	if err != nil {
		t.Fatalf("ListOutstandingForUser: %v", err)
	}
	if len(ids) != 2 || ids[0] != "charge-1" || ids[1] != "charge-2" {
		t.Fatalf("unexpected ids: %v", ids)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListOutstandingCandidates_ReturnsPendingIDs(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectQuery(`SELECT "id" FROM "wallet_sms_charges" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("charge-3"))

	ids, err := repo.ListOutstandingCandidates(context.Background(), 50, 0, 100)
	if err != nil {
		t.Fatalf("ListOutstandingCandidates: %v", err)
	}
	if len(ids) != 1 || ids[0] != "charge-3" {
		t.Fatalf("unexpected ids: %v", ids)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
