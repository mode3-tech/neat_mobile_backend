package vas

import (
	"context"
	"errors"
	"testing"

	"neat_mobile_app_backend/providers/baas"
	vasprovider "neat_mobile_app_backend/providers/vas"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

// fakeReconciliationVASProvider is a minimal VASService stub — only
// CheckStatus is exercised by the reconciliation sweep.
type fakeReconciliationVASProvider struct {
	checkStatusResp  *vasprovider.CheckStatusResponse
	checkStatusErr   error
	checkStatusCalls []string
}

func (f *fakeReconciliationVASProvider) FetchAllCategories(ctx context.Context) (*vasprovider.CategoriesResponse, error) {
	return nil, nil
}
func (f *fakeReconciliationVASProvider) FetchBillersByCategoryID(ctx context.Context, categoryID, page, size int) (*vasprovider.BillersByCategoryIDResponse, error) {
	return nil, nil
}
func (f *fakeReconciliationVASProvider) FetchProductsByCategoryIDAndBillerID(ctx context.Context, categoryID, billerID, page, size int) (*vasprovider.ProductResponse, error) {
	return nil, nil
}
func (f *fakeReconciliationVASProvider) GetAirtime(ctx context.Context, requestID, uniqueCode, phoneNumber string, amount int64) (*vasprovider.ISPResponse, error) {
	return nil, nil
}
func (f *fakeReconciliationVASProvider) GetData(ctx context.Context, requestID, uniqueCode, phoneNumber string, amount int64) (*vasprovider.ISPResponse, error) {
	return nil, nil
}
func (f *fakeReconciliationVASProvider) ValidateElectricity(ctx context.Context, requestID, uniqueCode, accountNumber string, accountType vasprovider.AccountType) (*vasprovider.ElectricityValidationResponse, error) {
	return nil, nil
}
func (f *fakeReconciliationVASProvider) PayElectricityBill(ctx context.Context, requestID, uniqueCode, accountNumber, name, address, phoneNumber string, accountType vasprovider.AccountType, amount int64) (*vasprovider.PayElectricityResponse, error) {
	return nil, nil
}
func (f *fakeReconciliationVASProvider) ValidateCable(ctx context.Context, requestID, uniqueCode, accountNumber string, noOfMonth int) (*vasprovider.CableValidationResponse, error) {
	return nil, nil
}
func (f *fakeReconciliationVASProvider) PayCableBill(ctx context.Context, requestID, uniqueCode, accountNumber, accountType, name, phoneNumber string, noOfMonth int, amount int64) (*vasprovider.PayCableResponse, error) {
	return nil, nil
}
func (f *fakeReconciliationVASProvider) CheckStatus(ctx context.Context, requestID string) (*vasprovider.CheckStatusResponse, error) {
	f.checkStatusCalls = append(f.checkStatusCalls, requestID)
	if f.checkStatusErr != nil {
		return nil, f.checkStatusErr
	}
	return f.checkStatusResp, nil
}
func (f *fakeReconciliationVASProvider) GetWalletBalance(ctx context.Context) (*vasprovider.WalletBalanceResponse, error) {
	return nil, nil
}

type fakeReconciliationWallet struct {
	wallet *CustomerWallet
	err    error
}

func (f *fakeReconciliationWallet) GetBalance(ctx context.Context, mobileUserID string) (*CustomerWallet, error) {
	return f.wallet, f.err
}

type txnStatusUpdate struct {
	txID         string
	balanceAfter int64
	status       TransactionStatus
}

type providerRefUpdate struct {
	txID        string
	providerRef string
}

type fakeReconciliationTxr struct {
	updates    []txnStatusUpdate
	refUpdates []providerRefUpdate
}

func (f *fakeReconciliationTxr) AddTransaction(ctx context.Context, transaction *Transaction) error {
	return nil
}

func (f *fakeReconciliationTxr) UpdateTransactionStatus(ctx context.Context, txID string, balanceAfter int64, status TransactionStatus) error {
	f.updates = append(f.updates, txnStatusUpdate{txID: txID, balanceAfter: balanceAfter, status: status})
	return nil
}

func (f *fakeReconciliationTxr) UpdateTransactionProviderReference(ctx context.Context, txID, providerRef string) error {
	f.refUpdates = append(f.refUpdates, providerRefUpdate{txID: txID, providerRef: providerRef})
	return nil
}

type fakeReconciliationBAAS struct {
	creditCalls int
	creditErr   error
}

func (f *fakeReconciliationBAAS) DebitCustomer(ctx context.Context, amount int64, customerID, referenceID string, metadata interface{}) (*baas.ProvidusWalletDebitResponse, error) {
	return nil, nil
}

func (f *fakeReconciliationBAAS) CreditCustomer(ctx context.Context, amount int64, referenceID, customerID string, metadata interface{}) (*baas.ProvidusWalletCreditResponse, error) {
	f.creditCalls++
	if f.creditErr != nil {
		return nil, f.creditErr
	}
	return &baas.ProvidusWalletCreditResponse{}, nil
}

func (f *fakeReconciliationBAAS) GetCustomerWallet(ctx context.Context, customerID string) (*baas.ProvidusCustomerWalletResponse, error) {
	return nil, nil
}

func TestReconcileAmbiguousOutcome_StalePending_ResolvesSuccess_BackfillsToken(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	vasProvider := &fakeReconciliationVASProvider{
		checkStatusResp: &vasprovider.CheckStatusResponse{
			Response: vasprovider.Response{ResponseCode: "00", ReferenceID: "ref-456"},
			Data: map[string]interface{}{
				"token": "1234-5678-9012-3456",
				"unit":  "42.5kWh",
			},
		},
	}
	txr := &fakeReconciliationTxr{}
	svc := NewService(repo, vasProvider, nil, txr, nil, nil, nil, nil)

	txn := &Transaction{
		ID:            "tx-1",
		MobileUserID:  "user-1",
		Amount:        50000,
		BalanceBefore: 100000,
		VASRequestID:  "req-1",
		Category:      TransactionCategoryElectricity,
		Status:        TransactionStatusPending,
	}

	mock.ExpectBegin()
	mock.ExpectExec(updateWalletTransactionsPattern()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc.reconcileAmbiguousOutcome(context.Background(), txn)

	if len(vasProvider.checkStatusCalls) != 1 || vasProvider.checkStatusCalls[0] != "req-1" {
		t.Fatalf("expected exactly one CheckStatus call with req-1, got %v", vasProvider.checkStatusCalls)
	}
	if len(txr.updates) != 1 {
		t.Fatalf("expected exactly one status update, got %d", len(txr.updates))
	}
	if txr.updates[0].status != TransactionStatusSuccessful {
		t.Fatalf("status = %s, want successful", txr.updates[0].status)
	}
	if want := txn.BalanceBefore - txn.Amount; txr.updates[0].balanceAfter != want {
		t.Fatalf("balanceAfter = %d, want %d", txr.updates[0].balanceAfter, want)
	}
	if len(txr.refUpdates) != 1 || txr.refUpdates[0].txID != "tx-1" || txr.refUpdates[0].providerRef != "ref-456" {
		t.Fatalf("expected provider reference update for tx-1 with ref-456, got %+v", txr.refUpdates)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations (token backfill not persisted): %v", err)
	}
}

func TestReconcileAmbiguousOutcome_ResolvesSuccess_NoReferenceID_SkipsProviderRefUpdate(t *testing.T) {
	repo, _, cleanup := newMockVASRepository(t)
	defer cleanup()

	vasProvider := &fakeReconciliationVASProvider{
		checkStatusResp: &vasprovider.CheckStatusResponse{
			Response: vasprovider.Response{ResponseCode: "00"},
		},
	}
	txr := &fakeReconciliationTxr{}
	svc := NewService(repo, vasProvider, nil, txr, nil, nil, nil, nil)

	txn := &Transaction{
		ID:            "tx-9",
		MobileUserID:  "user-9",
		Amount:        10000,
		BalanceBefore: 30000,
		VASRequestID:  "req-9",
		Category:      TransactionCategoryAirtime,
		Status:        TransactionStatusPending,
	}

	svc.reconcileAmbiguousOutcome(context.Background(), txn)

	if len(txr.refUpdates) != 0 {
		t.Fatalf("expected no provider reference update when ReferenceID is blank, got %+v", txr.refUpdates)
	}
}

func TestReconcileAmbiguousOutcome_ResolvesFailure_IssuesRefundExactlyOnce(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	vasProvider := &fakeReconciliationVASProvider{
		checkStatusResp: &vasprovider.CheckStatusResponse{Response: vasprovider.Response{ResponseCode: "99"}},
	}
	txr := &fakeReconciliationTxr{}
	baasClient := &fakeReconciliationBAAS{}
	wallet := &fakeReconciliationWallet{wallet: &CustomerWallet{WalletCustomerID: "cust-1"}}
	svc := NewService(repo, vasProvider, wallet, txr, baasClient, nil, nil, nil)

	txn := &Transaction{
		ID:            "tx-2",
		MobileUserID:  "user-2",
		Amount:        20000,
		BalanceBefore: 60000,
		VASRequestID:  "req-2",
		Category:      TransactionCategoryAirtime,
		Status:        TransactionStatusReversalPending,
	}

	mock.ExpectBegin()
	mock.ExpectExec(updateWalletTransactionsPattern()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc.reconcileAmbiguousOutcome(context.Background(), txn)

	if baasClient.creditCalls != 1 {
		t.Fatalf("CreditCustomer calls = %d, want exactly 1", baasClient.creditCalls)
	}
	if len(txr.updates) != 1 || txr.updates[0].status != TransactionStatusReversed {
		t.Fatalf("expected exactly one reversed update, got %+v", txr.updates)
	}
	if txr.updates[0].balanceAfter != txn.BalanceBefore {
		t.Fatalf("balanceAfter = %d, want balanceBefore %d restored", txr.updates[0].balanceAfter, txn.BalanceBefore)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations (wallet_refunded flag not persisted): %v", err)
	}
}

func TestReconcileAmbiguousOutcome_StillAmbiguous_TransitionsPendingToReversalPending(t *testing.T) {
	repo, _, cleanup := newMockVASRepository(t)
	defer cleanup()

	vasProvider := &fakeReconciliationVASProvider{
		checkStatusResp: &vasprovider.CheckStatusResponse{Response: vasprovider.Response{ResponseCode: "01"}},
	}
	txr := &fakeReconciliationTxr{}
	svc := NewService(repo, vasProvider, nil, txr, nil, nil, nil, nil)

	txn := &Transaction{
		ID:            "tx-3",
		Amount:        10000,
		BalanceBefore: 30000,
		VASRequestID:  "req-3",
		Status:        TransactionStatusPending,
	}

	svc.reconcileAmbiguousOutcome(context.Background(), txn)

	if len(txr.updates) != 1 || txr.updates[0].status != TransactionStatusReversalPending {
		t.Fatalf("expected exactly one reversal_pending update, got %+v", txr.updates)
	}
}

func TestReconcileAmbiguousOutcome_AlreadyReversalPending_NoRedundantUpdate(t *testing.T) {
	repo, _, cleanup := newMockVASRepository(t)
	defer cleanup()

	vasProvider := &fakeReconciliationVASProvider{
		checkStatusResp: &vasprovider.CheckStatusResponse{Response: vasprovider.Response{ResponseCode: "01"}},
	}
	txr := &fakeReconciliationTxr{}
	svc := NewService(repo, vasProvider, nil, txr, nil, nil, nil, nil)

	txn := &Transaction{
		ID:            "tx-4",
		Amount:        10000,
		BalanceBefore: 30000,
		VASRequestID:  "req-4",
		Status:        TransactionStatusReversalPending,
	}

	svc.reconcileAmbiguousOutcome(context.Background(), txn)

	if len(txr.updates) != 0 {
		t.Fatalf("expected no status update when already reversal_pending, got %+v", txr.updates)
	}
}

func TestReconcileAmbiguousOutcome_CheckStatusError_LeavesAsIs(t *testing.T) {
	repo, _, cleanup := newMockVASRepository(t)
	defer cleanup()

	vasProvider := &fakeReconciliationVASProvider{checkStatusErr: errors.New("provider unreachable")}
	txr := &fakeReconciliationTxr{}
	svc := NewService(repo, vasProvider, nil, txr, nil, nil, nil, nil)

	txn := &Transaction{ID: "tx-5", VASRequestID: "req-5", Status: TransactionStatusReversalPending}

	svc.reconcileAmbiguousOutcome(context.Background(), txn)

	if len(txr.updates) != 0 {
		t.Fatalf("expected no status update when CheckStatus errors, got %+v", txr.updates)
	}
}

func TestReconcileAmbiguousOutcome_NoPersistedRequestID_SkipsCheckStatus(t *testing.T) {
	repo, _, cleanup := newMockVASRepository(t)
	defer cleanup()

	vasProvider := &fakeReconciliationVASProvider{
		checkStatusResp: &vasprovider.CheckStatusResponse{Response: vasprovider.Response{ResponseCode: "00"}},
	}
	txr := &fakeReconciliationTxr{}
	svc := NewService(repo, vasProvider, nil, txr, nil, nil, nil, nil)

	txn := &Transaction{ID: "tx-6", VASRequestID: "", Status: TransactionStatusPending}

	svc.reconcileAmbiguousOutcome(context.Background(), txn)

	if len(vasProvider.checkStatusCalls) != 0 {
		t.Fatalf("expected CheckStatus not to be called for a pre-migration orphan, got %v", vasProvider.checkStatusCalls)
	}
	if len(txr.updates) != 0 {
		t.Fatalf("expected no status update for a pre-migration orphan, got %+v", txr.updates)
	}
}

func TestReconcileRefundPending_PartialCompletion_OnlyRetriesMissingHalf(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	txr := &fakeReconciliationTxr{}
	baasClient := &fakeReconciliationBAAS{}
	wallet := &fakeReconciliationWallet{wallet: &CustomerWallet{WalletCustomerID: "cust-1"}}
	svc := NewService(repo, nil, wallet, txr, baasClient, nil, nil, nil)

	txn := &Transaction{
		ID:             "tx-7",
		MobileUserID:   "user-7",
		Amount:         15000,
		BalanceBefore:  40000,
		UsedCashback:   true,
		CashbackAmount: 5000,
		WalletRefunded: true, // wallet portion already refunded by a previous partial attempt
		Status:         TransactionStatusRefundPending,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(lockUserPattern()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("user-7"))
	mock.ExpectQuery(findReservationByTxPattern()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(updateWalletTransactionsPattern()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc.reconcileRefundPending(context.Background(), txn)

	if baasClient.creditCalls != 0 {
		t.Fatalf("CreditCustomer calls = %d, want 0 (wallet portion already refunded)", baasClient.creditCalls)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestReconcileRefundPending_CashbackReleaseFails_StaysRefundPending(t *testing.T) {
	repo, mock, cleanup := newMockVASRepository(t)
	defer cleanup()

	txr := &fakeReconciliationTxr{}
	baasClient := &fakeReconciliationBAAS{}
	wallet := &fakeReconciliationWallet{wallet: &CustomerWallet{WalletCustomerID: "cust-1"}}
	svc := NewService(repo, nil, wallet, txr, baasClient, nil, nil, nil)

	txn := &Transaction{
		ID:             "tx-8",
		MobileUserID:   "user-8",
		Amount:         15000,
		BalanceBefore:  40000,
		UsedCashback:   true,
		CashbackAmount: 5000,
		WalletRefunded: true,
		Status:         TransactionStatusRefundPending,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(lockUserPattern()).
		WillReturnError(errors.New("db unavailable"))
	mock.ExpectRollback()

	svc.reconcileRefundPending(context.Background(), txn)

	if len(txr.updates) != 0 {
		t.Fatalf("expected no status update when refund is still incomplete, got %+v", txr.updates)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
