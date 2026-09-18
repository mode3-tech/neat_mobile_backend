package wallet

import (
	"context"
	"neat_mobile_app_backend/internal/modules/transaction"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockWalletService(t *testing.T) (*Service, sqlmock.Sqlmock, func()) {
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

	return &Service{repo: NewRepository(gormDB)}, mock, cleanup
}

func txRows(id, walletID string, status transaction.TransactionStatus) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "wallet_id", "status", "amount"}).
		AddRow(id, walletID, string(status), int64(50000))
}

func walletBalanceRows(internalWalletID string, availableBalance int64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"internal_wallet_id", "available_balance", "booked_balance"}).
		AddRow(internalWalletID, availableBalance, availableBalance)
}

func txLockPattern() string {
	return `SELECT \* FROM "wallet_transactions" WHERE`
}

func walletLockPattern() string {
	return `SELECT \* FROM "wallet_customer_wallets" WHERE`
}

func updateTxPattern() string {
	return `UPDATE "wallet_transactions" SET`
}

func updateWalletBalancePattern() string {
	return `UPDATE "wallet_customer_wallets" SET`
}

func TestInternalTransactionIDFrom(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     string
	}{
		{
			name:     "top-level key",
			metadata: map[string]interface{}{"internal_transaction_id": "tx-1"},
			want:     "tx-1",
		},
		{
			name: "nested under additionalMetadata",
			metadata: map[string]interface{}{
				"additionalMetadata": map[string]interface{}{"internal_transaction_id": "tx-2"},
			},
			want: "tx-2",
		},
		{
			name:     "absent",
			metadata: map[string]interface{}{"narration": "hello"},
			want:     "",
		},
		{
			name:     "nil metadata",
			metadata: nil,
			want:     "",
		},
		{
			name:     "blank top-level value falls through to empty",
			metadata: map[string]interface{}{"internal_transaction_id": ""},
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := internalTransactionIDFrom(tt.metadata); got != tt.want {
				t.Fatalf("internalTransactionIDFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestProcessCustomerBankTransfer_SuccessOnFailedTx_AppliesLateDebit is the
// core of the ambiguous-connection-drop fix: InitiateTransfer marked this tx
// Failed without ever debiting the wallet (CompleteDebitTransaction never
// ran), so a later webhook confirming success must apply the debit now, not
// just flip the status column.
func TestProcessCustomerBankTransfer_SuccessOnFailedTx_AppliesLateDebit(t *testing.T) {
	svc, mock, cleanup := newMockWalletService(t)
	defer cleanup()

	mock.ExpectQuery(txLockPattern()).
		WillReturnRows(txRows("tx-1", "wallet-1", transaction.TransactionStatusFailed))

	mock.ExpectBegin()
	mock.ExpectQuery(walletLockPattern()).
		WillReturnRows(walletBalanceRows("wallet-1", 100_000))
	mock.ExpectExec(updateTxPattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(updateWalletBalancePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.ProcessCustomerBankTransfer(context.Background(), &CustomerBankTransferData{
		TransactionReference: "provref-1",
		Status:               "success",
		Amount:               500,
		CustomerID:           "cust-1",
	})
	if err != nil {
		t.Fatalf("ProcessCustomerBankTransfer: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestProcessCustomerBankTransfer_SuccessOnFailedTx_FoundViaMetadataFallback
// covers the pure connection-drop case: provider_reference was never stored
// (no synchronous response ever came back), so the lookup must fall back to
// the internal_transaction_id stamped into the outgoing request's metadata.
func TestProcessCustomerBankTransfer_SuccessOnFailedTx_FoundViaMetadataFallback(t *testing.T) {
	svc, mock, cleanup := newMockWalletService(t)
	defer cleanup()

	mock.ExpectQuery(txLockPattern()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(txLockPattern()).
		WillReturnRows(txRows("tx-1", "wallet-1", transaction.TransactionStatusFailed))

	mock.ExpectBegin()
	mock.ExpectQuery(walletLockPattern()).
		WillReturnRows(walletBalanceRows("wallet-1", 100_000))
	mock.ExpectExec(updateTxPattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(updateWalletBalancePattern()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.ProcessCustomerBankTransfer(context.Background(), &CustomerBankTransferData{
		TransactionReference: "provref-unknown",
		Status:               "success",
		Amount:               500,
		CustomerID:           "cust-1",
		Metadata:             map[string]interface{}{"internal_transaction_id": "tx-1"},
	})
	if err != nil {
		t.Fatalf("ProcessCustomerBankTransfer: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestProcessCustomerBankTransfer_NoMatchEitherWay_NoOps confirms the webhook
// is dropped safely (no panic, no error) when neither lookup finds a row.
func TestProcessCustomerBankTransfer_NoMatchEitherWay_NoOps(t *testing.T) {
	svc, mock, cleanup := newMockWalletService(t)
	defer cleanup()

	mock.ExpectQuery(txLockPattern()).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.ProcessCustomerBankTransfer(context.Background(), &CustomerBankTransferData{
		TransactionReference: "provref-unknown",
		Status:               "success",
	})
	if err != nil {
		t.Fatalf("ProcessCustomerBankTransfer: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestProcessCustomerBankTransfer_FailureOnFailedTx_StaysNoOp confirms the
// reversal path stays safe for a tx that was already Failed locally:
// ReverseDebitTransaction no-ops on a non-Pending row since there was never
// a balance to reverse.
func TestProcessCustomerBankTransfer_FailureOnFailedTx_StaysNoOp(t *testing.T) {
	svc, mock, cleanup := newMockWalletService(t)
	defer cleanup()

	mock.ExpectQuery(txLockPattern()).
		WillReturnRows(txRows("tx-1", "wallet-1", transaction.TransactionStatusFailed))

	mock.ExpectBegin()
	mock.ExpectQuery(txLockPattern()).
		WillReturnRows(txRows("tx-1", "wallet-1", transaction.TransactionStatusFailed))
	mock.ExpectCommit()

	err := svc.ProcessCustomerBankTransfer(context.Background(), &CustomerBankTransferData{
		TransactionReference: "provref-1",
		Status:               "failed",
	})
	if err != nil {
		t.Fatalf("ProcessCustomerBankTransfer: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFundedBeneficiaryAccountNumber(t *testing.T) {
	tests := []struct {
		name string
		data AccountFundedData
		want string
	}{
		{
			name: "real transfer-in credits the beneficiary, not the payer",
			data: AccountFundedData{
				AccountNumber:            "9059375974", // payer (originator)
				BeneficiaryAccountNumber: "8891450872", // our customer
			},
			want: "8891450872",
		},
		{
			name: "documented single-account sample falls back to accountNumber",
			data: AccountFundedData{
				AccountNumber: "8869253566",
			},
			want: "8869253566",
		},
		{
			name: "blank/whitespace beneficiary falls back to accountNumber",
			data: AccountFundedData{
				AccountNumber:            "8869253566",
				BeneficiaryAccountNumber: "   ",
			},
			want: "8869253566",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fundedBeneficiaryAccountNumber(&tt.data); got != tt.want {
				t.Fatalf("fundedBeneficiaryAccountNumber() = %q, want %q", got, tt.want)
			}
		})
	}
}
