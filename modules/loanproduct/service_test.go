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

type stubCoreCustomerFinder struct {
	match *CoreCustomerMatchData
	err   error
	calls int
}

func (s *stubCoreCustomerFinder) MatchCustomerByBVN(context.Context, string) (*CoreCustomerMatchData, error) {
	s.calls++
	return s.match, s.err
}

type stubCoreLoanFinder struct {
	loans                 []CoreCustomerLoanItem
	customerLoansErr      error
	loanDetailErr         error
	customerLoansCalls    int
	loanDetailCalls       int
	loanRepaymentsCalls   int
	getLoanRepaymentsResp *[]LoanRepayment
}

func (s *stubCoreLoanFinder) GetCustomerLoans(context.Context, string) ([]CoreCustomerLoanItem, error) {
	s.customerLoansCalls++
	if s.customerLoansErr != nil {
		return nil, s.customerLoansErr
	}
	return s.loans, nil
}

func (s *stubCoreLoanFinder) GetLoanDetail(context.Context, string) (*CoreLoanDetail, error) {
	s.loanDetailCalls++
	if s.loanDetailErr != nil {
		return nil, s.loanDetailErr
	}
	return &CoreLoanDetail{}, nil
}

func (s *stubCoreLoanFinder) GetLoanRepayments(context.Context, string) (*[]LoanRepayment, error) {
	s.loanRepaymentsCalls++
	return s.getLoanRepaymentsResp, nil
}

func loanProductQueryPattern() string {
	return `SELECT .* FROM "wallet_loan_products" WHERE code = \$1 ORDER BY "wallet_loan_products"\."id" LIMIT \$2`
}

func loanRuleQueryPattern() string {
	return `SELECT .* FROM "wallet_loan_product_rules" WHERE product_id = \$1 ORDER BY "wallet_loan_product_rules"\."id" LIMIT \$2`
}

func insertLoanApplicationQueryPattern() string {
	return `INSERT INTO "wallet_loan_applications"`
}

func expectApplyForLoanProductAndRuleQueries(mock sqlmock.Sqlmock, productID string) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	productRows := sqlmock.NewRows([]string{
		"id",
		"code",
		"name",
		"description",
		"min_loan_amount",
		"max_loan_amount",
		"interest_rate_bps",
		"repayment_frequency",
		"grace_period_days",
		"loan_term_value",
		"late_penalty_bps",
		"allows_concurrent_loans",
		"is_active",
		"created_at",
		"updated_at",
	}).AddRow(productID, string(LoanTypeBusiness), "Biz Loan", "Business loan", int64(1000), int64(500000), 24, string(LoanFrequencyWeekly), 0, 8, 0, false, true, now, now)

	mock.ExpectQuery(loanProductQueryPattern()).
		WithArgs(LoanTypeBusiness, 1).
		WillReturnRows(productRows)

	requireBVN := true
	requireNIN := true
	requirePhone := true
	requireNoDefault := true

	ruleRows := sqlmock.NewRows([]string{
		"id",
		"product_id",
		"min_savings_balance",
		"min_account_age_days",
		"max_account_loans",
		"require_kyc",
		"require_bvn",
		"require_nin",
		"require_phone_verified",
		"require_no_outstanding_default",
		"high_value_threshold",
		"branch_manager_approval_limit",
	}).AddRow("rule-1", productID, int64(0), 0, 2, false, requireBVN, requireNIN, requirePhone, requireNoDefault, 100000, int64(500000))

	mock.ExpectQuery(loanRuleQueryPattern()).
		WithArgs(productID, 1).
		WillReturnRows(ruleRows)
}

func expectApplyForLoanInsert(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec(insertLoanApplicationQueryPattern()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
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

	service := NewService(repo, nil, nil, nil, nil, nil)

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

	service := NewService(repo, nil, nil, nil, nil, nil)

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

	service := NewService(repo, nil, nil, nil, nil, nil)

	_, err := service.ApplyForLoan(context.Background(), LoanRequest{TransactionPin: "1234"}, "user-1")
	if !errors.Is(err, ErrTransactionPinTemporarilyLocked) {
		t.Fatalf("expected ErrTransactionPinTemporarilyLocked, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestServiceApplyForLoan_NoCoreCustomerMatchStillCreatesApplication(t *testing.T) {
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
	}).AddRow(nil, "08012345678", dob, "12345678901", "12345678901", true, true, true, hashTestPin(t, "1234"), 0, nil)

	mock.ExpectQuery(getUserQueryPattern()).
		WithArgs("user-1", 1).
		WillReturnRows(rows)

	expectApplyForLoanProductAndRuleQueries(mock, "product-1")
	expectApplyForLoanInsert(mock)

	customerFinder := &stubCoreCustomerFinder{
		match: &CoreCustomerMatchData{MatchStatus: CoreCustomerNoMatch},
	}
	loanFinder := &stubCoreLoanFinder{
		customerLoansErr: errors.New("unexpected core loan lookup"),
		loanDetailErr:    errors.New("unexpected core loan detail lookup"),
	}

	service := NewService(repo, customerFinder, loanFinder, nil, nil, nil)

	resp, err := service.ApplyForLoan(context.Background(), LoanRequest{
		LoanProductType:   LoanTypeBusiness,
		BusinessAddress:   "12 Marina",
		BusinessStartDate: "2020-01",
		BusinessValue:     "500000",
		LoanAmount:        "150000",
		TransactionPin:    "1234",
	}, "user-1")
	if err != nil {
		t.Fatalf("ApplyForLoan returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("ApplyForLoan returned nil response")
	}
	if resp.LoanStatus != LoanStatusPending {
		t.Fatalf("unexpected loan status: %q", resp.LoanStatus)
	}
	if customerFinder.calls != 1 {
		t.Fatalf("expected one customer match lookup, got %d", customerFinder.calls)
	}
	if loanFinder.customerLoansCalls != 0 {
		t.Fatalf("expected no core loan lookup, got %d", loanFinder.customerLoansCalls)
	}
	if loanFinder.loanDetailCalls != 0 {
		t.Fatalf("expected no core loan detail lookup, got %d", loanFinder.loanDetailCalls)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestServiceApplyForLoan_SingleCoreCustomerMatchPersistsIDAndChecksLoans(t *testing.T) {
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
	}).AddRow(nil, "08012345678", dob, "12345678901", "12345678901", true, true, true, hashTestPin(t, "1234"), 0, nil)

	mock.ExpectQuery(getUserQueryPattern()).
		WithArgs("user-1", 1).
		WillReturnRows(rows)

	expectApplyForLoanProductAndRuleQueries(mock, "product-1")

	mock.ExpectBegin()
	mock.ExpectExec(updateUserCoreCustomerIDQueryPattern()).
		WithArgs("2048", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	expectApplyForLoanInsert(mock)

	customerFinder := &stubCoreCustomerFinder{
		match: &CoreCustomerMatchData{
			MatchStatus: CoreCustomerSingleMatch,
			Customer: &CoreMatchedCustomer{
				CustomerID: "2048",
				FullName:   "Jane Doe",
			},
		},
	}
	loanFinder := &stubCoreLoanFinder{
		loans: []CoreCustomerLoanItem{},
	}

	service := NewService(repo, customerFinder, loanFinder, nil, nil, nil)

	resp, err := service.ApplyForLoan(context.Background(), LoanRequest{
		LoanProductType:   LoanTypeBusiness,
		BusinessAddress:   "12 Marina",
		BusinessStartDate: "2020-01",
		BusinessValue:     "500000",
		LoanAmount:        "150000",
		TransactionPin:    "1234",
	}, "user-1")
	if err != nil {
		t.Fatalf("ApplyForLoan returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("ApplyForLoan returned nil response")
	}
	if customerFinder.calls != 1 {
		t.Fatalf("expected one customer match lookup, got %d", customerFinder.calls)
	}
	if loanFinder.customerLoansCalls != 1 {
		t.Fatalf("expected one core loan lookup, got %d", loanFinder.customerLoansCalls)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
