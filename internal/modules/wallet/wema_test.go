package wallet

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"
	"neat_mobile_app_backend/internal/modules/transaction"

	"github.com/DATA-DOG/go-sqlmock"
)

// fakeSigner "signs" by hex-encoding, which is enough to test the wiring
// (and, like the real base64url HMAC, never contains a "."); the real HMAC is
// tested in internal/adapters/wema.
type fakeSigner struct{}

func (fakeSigner) Sign(purpose, value string) (string, error) {
	return fmt.Sprintf("%x", purpose+"|"+value), nil
}
func (fakeSigner) Verify(purpose, value, sig string) bool {
	want, _ := fakeSigner{}.Sign(purpose, value)
	return sig == want
}

type fakeWema struct {
	debitStatus  ProviderTransferStatus
	creditStatus ProviderTransferStatus
	statusErr    error
	statementRef string
	checked      []string
}

func (f *fakeWema) CheckDebitStatus(_ context.Context, ref string) (ProviderTransferStatus, error) {
	f.checked = append(f.checked, "debit:"+ref)
	return f.debitStatus, f.statusErr
}
func (f *fakeWema) CheckCreditStatus(_ context.Context, ref string) (ProviderTransferStatus, error) {
	f.checked = append(f.checked, "credit:"+ref)
	return f.creditStatus, f.statusErr
}
func (f *fakeWema) FundWallet(context.Context, string, float64, string, string) (*TransferResponse, error) {
	return &TransferResponse{Status: true}, nil
}
func (f *fakeWema) TransactionHistory(context.Context, string, time.Time, time.Time, string) ([]ProviderTransaction, error) {
	return nil, nil
}
func (f *fakeWema) InitiateStatement(context.Context, string, time.Time, time.Time) (string, error) {
	return f.statementRef, nil
}
func (f *fakeWema) StatementTransactions(_ context.Context, ref string) ([]ProviderTransaction, error) {
	return []ProviderTransaction{{ReferenceID: ref}}, nil
}

func newWemaTestService(t *testing.T, wema *fakeWema) (*Service, sqlmock.Sqlmock, func()) {
	t.Helper()
	svc, mock, cleanup := newMockWalletService(t)
	svc.ConfigureWema(wema, fakeSigner{})
	svc.auditLogger = noopAuditLogger{}
	return svc, mock, cleanup
}

func walletRow(mobileUserID, provider, accountNumber string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"mobile_user_id", "internal_wallet_id", "provider", "account_number"}).
		AddRow(mobileUserID, "iw-1", provider, accountNumber)
}

func TestIsWemaProvider(t *testing.T) {
	for _, p := range []string{"wema", "alat", " WEMA ", "Alat"} {
		if !IsWemaProvider(p) {
			t.Errorf("IsWemaProvider(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"", "providus", "optimus"} {
		if IsWemaProvider(p) {
			t.Errorf("IsWemaProvider(%q) = true, want false", p)
		}
	}
}

func TestCustomerLookupKey(t *testing.T) {
	if got := customerLookupKey(&CustomerWallet{Provider: "wema", AccountNumber: "0123", WalletCustomerID: "0803"}); got != "0123" {
		t.Errorf("wema lookup key = %q, want account number", got)
	}
	if got := customerLookupKey(&CustomerWallet{Provider: "providus", AccountNumber: "0123", WalletCustomerID: "cust-1"}); got != "cust-1" {
		t.Errorf("providus lookup key = %q, want customer ID", got)
	}
}

func TestAuthorizeAlatTransaction(t *testing.T) {
	sig := "ignored" // the callback no longer verifies securityInfo

	t.Run("signed pending transaction is approved", func(t *testing.T) {
		svc, mock, cleanup := newWemaTestService(t, &fakeWema{})
		defer cleanup()
		mock.ExpectQuery(txLockPattern()).WillReturnRows(txRows("tx-1", "iw-1", transaction.TransactionStatusPending))
		if !svc.AuthorizeAlatTransaction(context.Background(), " tx-1 ", sig) {
			t.Fatal("expected approval")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("signed but already settled transaction is rejected", func(t *testing.T) {
		svc, mock, cleanup := newWemaTestService(t, &fakeWema{})
		defer cleanup()
		mock.ExpectQuery(txLockPattern()).WillReturnRows(txRows("tx-1", "iw-1", transaction.TransactionStatusFailed))
		if svc.AuthorizeAlatTransaction(context.Background(), "tx-1", sig) {
			t.Fatal("expected rejection")
		}
	})

	t.Run("unknown transaction is rejected", func(t *testing.T) {
		svc, mock, cleanup := newWemaTestService(t, &fakeWema{})
		defer cleanup()
		mock.ExpectQuery(txLockPattern()).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		if svc.AuthorizeAlatTransaction(context.Background(), "tx-1", sig) {
			t.Fatal("expected rejection")
		}
	})

	t.Run("dashless UUID reference is matched to the dashed transaction ID", func(t *testing.T) {
		svc, mock, cleanup := newWemaTestService(t, &fakeWema{})
		defer cleanup()
		const id = "e6cda7ad-e1cb-65e2-c239-a5170fb894f3"
		mock.ExpectQuery(txLockPattern()).WithArgs(id, sqlmock.AnyArg()).WillReturnRows(txRows(id, "iw-1", transaction.TransactionStatusPending))
		if !svc.AuthorizeAlatTransaction(context.Background(), "e6cda7ade1cb65e2c239a5170fb894f3", "anything") {
			t.Fatal("expected approval")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestReconcileWemaTransaction(t *testing.T) {
	pendingDebit := func() *transaction.Transaction {
		return &transaction.Transaction{ID: "tx-1", WalletID: "iw-1", ProviderReference: "tx-1", Type: transaction.TransactionTypeDebit, Status: transaction.TransactionStatusPending, Amount: 50000}
	}
	pendingCredit := func() *transaction.Transaction {
		tx := pendingDebit()
		tx.Type = transaction.TransactionTypeCredit
		return tx
	}

	t.Run("successful debit is marked successful", func(t *testing.T) {
		wema := &fakeWema{debitStatus: ProviderTransferSuccessful}
		svc, mock, cleanup := newWemaTestService(t, wema)
		defer cleanup()
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "wallet_transactions" SET "status"=\$1,"updated_at"=\$2 WHERE id = \$3 AND status = \$4`).
			WithArgs(transaction.TransactionStatusSuccessful, sqlmock.AnyArg(), "tx-1", transaction.TransactionStatusPending).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		svc.reconcileWemaTransaction(context.Background(), pendingDebit())
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		if len(wema.checked) != 1 || wema.checked[0] != "debit:tx-1" {
			t.Errorf("checked = %v, want [debit:tx-1]", wema.checked)
		}
	})

	t.Run("failed debit is reversed", func(t *testing.T) {
		svc, mock, cleanup := newWemaTestService(t, &fakeWema{debitStatus: ProviderTransferFailed})
		defer cleanup()
		mock.ExpectBegin()
		mock.ExpectQuery(txLockPattern()).WillReturnRows(txRows("tx-1", "iw-1", transaction.TransactionStatusPending))
		mock.ExpectQuery(walletLockPattern()).WillReturnRows(walletBalanceRows("iw-1", 100000))
		mock.ExpectExec(updateTxPattern()).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateWalletBalancePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		svc.reconcileWemaTransaction(context.Background(), pendingDebit())
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("successful payout credits the wallet", func(t *testing.T) {
		wema := &fakeWema{creditStatus: ProviderTransferSuccessful}
		svc, mock, cleanup := newWemaTestService(t, wema)
		defer cleanup()
		mock.ExpectBegin()
		mock.ExpectQuery(txLockPattern()).WillReturnRows(txRows("tx-1", "iw-1", transaction.TransactionStatusPending))
		mock.ExpectQuery(walletLockPattern()).WillReturnRows(walletBalanceRows("iw-1", 100000))
		mock.ExpectExec(updateTxPattern()).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateWalletBalancePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		svc.reconcileWemaTransaction(context.Background(), pendingCredit())
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		if len(wema.checked) != 1 || wema.checked[0] != "credit:tx-1" {
			t.Errorf("checked = %v, want [credit:tx-1]", wema.checked)
		}
	})

	t.Run("failed payout is marked failed without touching the wallet", func(t *testing.T) {
		svc, mock, cleanup := newWemaTestService(t, &fakeWema{creditStatus: ProviderTransferFailed})
		defer cleanup()
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "wallet_transactions" SET "status"=\$1,"updated_at"=\$2 WHERE id = \$3 AND status = \$4`).
			WithArgs(transaction.TransactionStatusFailed, sqlmock.AnyArg(), "tx-1", transaction.TransactionStatusPending).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		svc.reconcileWemaTransaction(context.Background(), pendingCredit())
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	for name, wema := range map[string]*fakeWema{
		"still pending":      {debitStatus: ProviderTransferPending},
		"status check error": {statusErr: errors.New("boom")},
	} {
		t.Run(name+" only records the attempt", func(t *testing.T) {
			svc, mock, cleanup := newWemaTestService(t, wema)
			defer cleanup()
			mock.ExpectBegin()
			mock.ExpectExec(`UPDATE "wallet_transactions" SET .*"reconciliation_attempts"=reconciliation_attempts \+ 1`).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			svc.reconcileWemaTransaction(context.Background(), pendingDebit())
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReconcilePendingWemaTransactions_NoopWithoutWema(t *testing.T) {
	svc, mock, cleanup := newMockWalletService(t)
	defer cleanup()
	if err := svc.ReconcilePendingWemaTransactions(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWemaStatement_IDIsBoundToUser(t *testing.T) {
	ctx := context.Background()
	from, to := time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 8, 31, 0, 0, 0, 0, time.UTC)

	svc, mock, cleanup := newWemaTestService(t, &fakeWema{statementRef: "STMT.1"})
	defer cleanup()

	mock.ExpectQuery(walletLockPattern()).WillReturnRows(walletRow("user-a", "wema", "0123"))
	id, err := svc.InitiateWemaStatement(ctx, "user-a", from, to)
	if err != nil {
		t.Fatal(err)
	}

	// The owner can fetch it; the reference may itself contain dots.
	mock.ExpectQuery(walletLockPattern()).WillReturnRows(walletRow("user-a", "wema", "0123"))
	items, err := svc.GetWemaStatement(ctx, "user-a", id)
	if err != nil {
		t.Fatalf("owner fetch: %v", err)
	}
	if len(items) != 1 || items[0].ReferenceID != "STMT.1" {
		t.Errorf("items = %+v, want reference STMT.1", items)
	}

	// Another user can't, nor can a tampered ID.
	for user, sid := range map[string]string{"user-b": id, "user-a": "STMT.1.forged"} {
		mock.ExpectQuery(walletLockPattern()).WillReturnRows(walletRow(user, "wema", "0999"))
		if _, err := svc.GetWemaStatement(ctx, user, sid); !errors.Is(err, appErr.ErrInvalidStatementID) {
			t.Errorf("GetWemaStatement(%s, %s) err = %v, want ErrInvalidStatementID", user, sid, err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWemaFeatures_RequireWemaWallet(t *testing.T) {
	svc, mock, cleanup := newWemaTestService(t, &fakeWema{})
	defer cleanup()
	mock.ExpectQuery(walletLockPattern()).WillReturnRows(walletRow("user-a", "providus", "0123"))
	_, err := svc.GetWemaTransactionHistory(context.Background(), "user-a", time.Now(), time.Now(), "")
	if !errors.Is(err, appErr.ErrWemaWalletRequired) {
		t.Fatalf("err = %v, want ErrWemaWalletRequired", err)
	}
}

func TestParseDateRange(t *testing.T) {
	from, to, err := parseDateRange("2024-08-01", "2024-08-31")
	if err != nil {
		t.Fatal(err)
	}
	if !from.Equal(time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)) || !to.Equal(time.Date(2024, 8, 31, 23, 59, 59, 0, time.UTC)) {
		t.Errorf("got %s - %s", from, to)
	}
	if _, _, err := parseDateRange("2024-08-31", "2024-08-01"); err == nil {
		t.Error("expected an error for to < from")
	}
	if _, _, err := parseDateRange("yesterday", "2024-08-01"); err == nil {
		t.Error("expected an error for an unparseable date")
	}
}

type noopAuditLogger struct{}

func (noopAuditLogger) CreateAuditLog(context.Context, *auditlog.AuditLog) error { return nil }
