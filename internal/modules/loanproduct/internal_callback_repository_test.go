package loanproduct

import (
	"context"
	"neat_mobile_app_backend/internal/crypto"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func testCipher(t *testing.T) *crypto.FieldCipher {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	c, err := crypto.NewFieldCipher(key)
	if err != nil {
		t.Fatalf("NewFieldCipher: %v", err)
	}
	return c
}

func linkWalletUserCoreCustomerIDByBVNQueryPattern() string {
	return regexp.QuoteMeta(`UPDATE "wallet_users" SET "core_customer_id"=$1 WHERE bvn_hash = $2`)
}

func getLoanApplicationBVNRecordForCBAQueryPattern() string {
	return `(?s)SELECT .*wallet_bvn_records\.bvn.* FROM "wallet_loan_applications" INNER JOIN wallet_users ON wallet_users\.id = wallet_loan_applications\.mobile_user_id INNER JOIN wallet_bvn_records ON wallet_bvn_records\.bvn_hash = wallet_users\.bvn_hash WHERE wallet_loan_applications\.mobile_user_id = \$1 ORDER BY wallet_loan_applications\.created_at DESC LIMIT \$2`
}

func getMostRecentEmbryoLoanApplicationForCBAQueryPattern() string {
	return `(?s)SELECT .*wallet_loan_applications\.application_ref.*wallet_users\.customer_status AS user_customer_status.* FROM "wallet_loan_applications" LEFT JOIN wallet_users ON wallet_users\.id = wallet_loan_applications\.mobile_user_id LEFT JOIN wallet_bvn_records ON wallet_bvn_records\.bvn_hash = wallet_users\.bvn_hash WHERE wallet_loan_applications\.mobile_user_id = \$1 AND \(\(?wallet_loan_applications\.loan_status = \$2 OR wallet_users\.customer_status = \$3\)?\) ORDER BY wallet_loan_applications\.created_at DESC LIMIT \$4`
}

func listEmbryoLoanApplicationSummariesForCBAQueryPattern() string {
	return `(?s)SELECT .*wallet_bvn_records\.first_name.*wallet_users\.customer_status.* FROM "wallet_loan_applications" LEFT JOIN wallet_users ON wallet_users\.id = wallet_loan_applications\.mobile_user_id LEFT JOIN wallet_bvn_records ON wallet_bvn_records\.bvn_hash = wallet_users\.bvn_hash WHERE \(\(?wallet_loan_applications\.loan_status = \$1 OR wallet_users\.customer_status = \$2\)?\) ORDER BY wallet_loan_applications\.created_at DESC LIMIT \$3 OFFSET \$4`
}

func countEmbryoLoanApplicationSummariesForCBAQueryPattern() string {
	return `(?s)SELECT count\(\*\) FROM "wallet_loan_applications" LEFT JOIN wallet_users ON wallet_users\.id = wallet_loan_applications\.mobile_user_id WHERE \(\(?wallet_loan_applications\.loan_status = \$1 OR wallet_users\.customer_status = \$2\)?\)`
}

func cbaApplicationReadColumns() []string {
	return []string{
		"application_ref",
		"mobile_user_id",
		"application_core_customer_id",
		"user_core_customer_id",
		"user_customer_status",
		"phone_number",
		"loan_product_type",
		"business_address",
		"business_value",
		"business_type",
		"requested_amount",
		"loan_status",
		"tenure",
		"tenure_value",
		"bvn_record_id",
		"bvn",
		"first_name",
		"middle_name",
		"last_name",
		"gender",
		"nationality",
		"state_of_origin",
		"date_of_birth",
		"email_address",
		"mobile_phone",
		"alternative_mobile_phone",
		"bank_name",
		"full_home_address",
		"passport_on_bvn",
		"city",
		"landmark",
	}
}

func TestInternalRepository_LinkWalletUserCoreCustomerIDByBVN_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	internalRepo := NewInternalRepository(repo.db, testCipher(t))

	mock.ExpectBegin()
	mock.ExpectExec(linkWalletUserCoreCustomerIDByBVNQueryPattern()).
		WithArgs("2048", crypto.Hash("12345678901")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	linkedUsers, err := internalRepo.LinkWalletUserCoreCustomerIDByBVN(context.Background(), "12345678901", "2048")
	if err != nil {
		t.Fatalf("LinkWalletUserCoreCustomerIDByBVN returned error: %v", err)
	}
	if linkedUsers != 1 {
		t.Fatalf("linkedUsers = %d, want 1", linkedUsers)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestInternalRepository_GetLoanApplicationBVNRecordForCBA_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	dob := time.Date(1995, 7, 10, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"application_ref",
		"bvn",
		"first_name",
		"middle_name",
		"last_name",
		"gender",
		"nationality",
		"state_of_origin",
		"date_of_birth",
		"email_address",
		"mobile_phone",
		"alternative_mobile_phone",
		"bank_name",
		"full_home_address",
		"passport_on_bvn",
		"city",
		"landmark",
	}).AddRow(
		"app-ref-123",
		"12345678901",
		"Jane",
		"Mary",
		"Doe",
		"female",
		"Nigerian",
		"Lagos",
		dob,
		"jane@example.com",
		"08012345678",
		"08087654321",
		"Bank Plc",
		"12 Allen Avenue",
		"https://example.com/passport.jpg",
		"Ikeja",
		"Under bridge",
	)

	mock.ExpectQuery(getLoanApplicationBVNRecordForCBAQueryPattern()).
		WithArgs("user-1", 1).
		WillReturnRows(rows)

	internalRepo := NewInternalRepository(repo.db, testCipher(t))

	record, err := internalRepo.GetLoanApplicationBVNRecordForCBA(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetLoanApplicationBVNRecordForCBA returned error: %v", err)
	}
	if record == nil {
		t.Fatal("GetLoanApplicationBVNRecordForCBA returned nil record")
	}
	if record.ApplicationRef != "app-ref-123" {
		t.Fatalf("unexpected application ref: %q", record.ApplicationRef)
	}
	if record.BVN == nil || *record.BVN != "12345678901" {
		t.Fatalf("unexpected bvn: %#v", record.BVN)
	}
	if record.City == nil || *record.City != "Ikeja" {
		t.Fatalf("unexpected city: %#v", record.City)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
