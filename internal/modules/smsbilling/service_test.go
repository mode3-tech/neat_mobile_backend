package smsbilling

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUnitsFor(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    int
	}{
		{"empty", "", 1},
		{"one segment", strings.Repeat("a", 160), 1},
		{"just over one segment", strings.Repeat("a", 161), 2},
		{"exactly two segments", strings.Repeat("a", 320), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := unitsFor(tc.message); got != tc.want {
				t.Fatalf("unitsFor(len=%d) = %d, want %d", len(tc.message), got, tc.want)
			}
		})
	}
}

// TestChargeAndCollect_RecordsThenImmediatelyAttemptsCollection exercises the
// path called right after a billable SMS is sent: it must create the charge
// row and then try to collect it in the same call, so a customer with funds
// gets charged without waiting for the sweep.
func TestChargeAndCollect_RecordsThenImmediatelyAttemptsCollection(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	svc := NewService(repo, DefaultConfig(400))

	// CreateCharge
	mock.ExpectBegin()
	mock.ExpectExec(insertChargePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// AttemptCollect - wallet has sufficient funds
	mock.ExpectBegin()
	mock.ExpectQuery(lockChargePattern()).
		WillReturnRows(chargeRows("ignored-by-mock", "user-1", string(SMSChargeStatusPending), 400, 0))
	mock.ExpectQuery(lockWalletPattern()).
		WillReturnRows(walletRows("wallet-1", "user-1", 10_000, 10_000))
	mock.ExpectExec(insertLedgerTxPattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(updateWalletBalancePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(updateChargePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc.ChargeAndCollect(context.Background(), "user-1", "+2348000000000", "hello", "login")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestCollectOutstanding_AttemptsEveryPendingChargeForUser is the
// credit-triggered recovery path (wallet.Service.ProcessAccountFunded calls
// this after a deposit lands).
func TestCollectOutstanding_AttemptsEveryPendingChargeForUser(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	svc := NewService(repo, DefaultConfig(400))

	mock.ExpectQuery(`SELECT "id" FROM "wallet_sms_charges" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("charge-1").AddRow("charge-2"))

	for i := 0; i < 2; i++ {
		mock.ExpectBegin()
		mock.ExpectQuery(lockChargePattern()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "phone", "purpose", "units", "unit_price_kobo", "amount_kobo", "status", "reference", "attempts"}))
		mock.ExpectCommit()
	}

	svc.CollectOutstanding(context.Background(), "user-1")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestReconcileOutstanding_AttemptsEveryCandidate is the cron sweep entrypoint.
func TestReconcileOutstanding_AttemptsEveryCandidate(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	svc := NewService(repo, DefaultConfig(400))

	mock.ExpectQuery(`SELECT "id" FROM "wallet_sms_charges" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("charge-1"))

	mock.ExpectBegin()
	mock.ExpectQuery(lockChargePattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mobile_user_id", "phone", "purpose", "units", "unit_price_kobo", "amount_kobo", "status", "reference", "attempts"}))
	mock.ExpectCommit()

	if err := svc.ReconcileOutstanding(context.Background()); err != nil {
		t.Fatalf("ReconcileOutstanding: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
