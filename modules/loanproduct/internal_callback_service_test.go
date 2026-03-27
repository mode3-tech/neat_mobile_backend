package loanproduct

import (
	"context"
	"errors"
	"neat_mobile_app_backend/models"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

func TestInternalService_GetLoanApplicationBVNRecordForCBA_InvalidMobileUserID(t *testing.T) {
	repo, _, cleanup := newMockRepository(t)
	defer cleanup()

	service := NewInternalService(NewInternalRepository(repo.db))

	_, err := service.GetLoanApplicationBVNRecordForCBA(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidMobileUserID) {
		t.Fatalf("expected ErrInvalidMobileUserID, got %v", err)
	}
}

func TestInternalService_GetLoanApplicationBVNRecordForCBA_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectQuery(getLoanApplicationBVNRecordForCBAQueryPattern()).
		WithArgs("missing-user", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	service := NewInternalService(NewInternalRepository(repo.db))

	_, err := service.GetLoanApplicationBVNRecordForCBA(context.Background(), "missing-user")
	if !errors.Is(err, ErrCustomerRecordNotFound) {
		t.Fatalf("expected ErrCustomerRecordNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestInternalService_GetLoanApplicationsForCBA_ReturnsEmptyWhenNoEmbryoApplicationExists(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	mock.ExpectQuery(getMostRecentEmbryoLoanApplicationForCBAQueryPattern()).
		WithArgs("user-1", LoanStatusEmbryo, models.CustomerStatusEmbryo, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	service := NewInternalService(NewInternalRepository(repo.db))

	resp, err := service.GetLoanApplicationsForCBA(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetLoanApplicationsForCBA returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("GetLoanApplicationsForCBA returned nil response")
	}
	if resp.Count != 0 {
		t.Fatalf("count = %d, want 0", resp.Count)
	}
	if len(resp.Applications) != 0 {
		t.Fatalf("applications len = %d, want 0", len(resp.Applications))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestInternalService_GetLoanApplicationsForCBA_ReturnsLatestEligibleApplication(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	rows := sqlmock.NewRows(cbaApplicationReadColumns()).AddRow(
		"app-ref-123",
		"user-1",
		nil,
		"2048",
		"embryo",
		"08012345678",
		"BUSINESS-WK",
		"12 Allen Avenue",
		int64(500000),
		"Retail",
		int64(150000),
		"active",
		"weekly",
		4,
		"bvn-record-1",
		"12345678901",
		"Jane",
		"",
		"Doe",
		"female",
		"Nigerian",
		"Lagos",
		nil,
		"jane@example.com",
		"08012345678",
		nil,
		"Bank Plc",
		"12 Allen Avenue",
		"https://example.com/passport.jpg",
		nil,
		nil,
	)

	mock.ExpectQuery(getMostRecentEmbryoLoanApplicationForCBAQueryPattern()).
		WithArgs("user-1", LoanStatusEmbryo, models.CustomerStatusEmbryo, 1).
		WillReturnRows(rows)

	service := NewInternalService(NewInternalRepository(repo.db))

	resp, err := service.GetLoanApplicationsForCBA(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetLoanApplicationsForCBA returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("GetLoanApplicationsForCBA returned nil response")
	}
	if resp.Count != 1 {
		t.Fatalf("count = %d, want 1", resp.Count)
	}
	if len(resp.Applications) != 1 {
		t.Fatalf("applications len = %d, want 1", len(resp.Applications))
	}
	if resp.Applications[0].Loan.LoanStatus != "active" {
		t.Fatalf("loan status = %q, want active", resp.Applications[0].Loan.LoanStatus)
	}
	if resp.Applications[0].Loan.Name != "Jane Doe" {
		t.Fatalf("name = %q, want Jane Doe", resp.Applications[0].Loan.Name)
	}
	if resp.Applications[0].Loan.CoreCustomerID == nil || *resp.Applications[0].Loan.CoreCustomerID != "2048" {
		t.Fatalf("unexpected core customer id: %#v", resp.Applications[0].Loan.CoreCustomerID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestInternalService_GetEmbryoLoanApplicationsForCBA_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepository(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{
		"application_ref",
		"mobile_user_id",
		"phone_number",
		"first_name",
		"middle_name",
		"last_name",
		"gender",
		"loan_status",
		"customer_status",
	}).AddRow("app-ref-1", "user-1", "08012345678", "Jane", "", "Doe", "female", "embryo", "embryo").
		AddRow("app-ref-2", "user-2", "08087654321", "John", "M", "Smith", nil, "active", "embryo")

	mock.ExpectQuery(listEmbryoLoanApplicationSummariesForCBAQueryPattern()).
		WithArgs(LoanStatusEmbryo, models.CustomerStatusEmbryo).
		WillReturnRows(rows)

	service := NewInternalService(NewInternalRepository(repo.db))

	resp, err := service.GetEmbryoLoanApplicationsForCBA(context.Background())
	if err != nil {
		t.Fatalf("GetEmbryoLoanApplicationsForCBA returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("GetEmbryoLoanApplicationsForCBA returned nil response")
	}
	if resp.Count != 2 {
		t.Fatalf("count = %d, want 2", resp.Count)
	}
	if resp.Applications[0].Name != "Jane Doe" {
		t.Fatalf("first name = %q, want Jane Doe", resp.Applications[0].Name)
	}
	if resp.Applications[0].ApplicationRef != "app-ref-1" {
		t.Fatalf("first application ref = %q, want app-ref-1", resp.Applications[0].ApplicationRef)
	}
	if resp.Applications[1].MobileUserID != "user-2" {
		t.Fatalf("second mobile user id = %q, want user-2", resp.Applications[1].MobileUserID)
	}
	if resp.Applications[0].Gender != "female" {
		t.Fatalf("first gender = %q, want female", resp.Applications[0].Gender)
	}
	if resp.Applications[1].PhoneNumber != "08087654321" {
		t.Fatalf("second phone number = %q, want 08087654321", resp.Applications[1].PhoneNumber)
	}
	if resp.Applications[1].LoanStatus != "active" {
		t.Fatalf("second loan status = %q, want active", resp.Applications[1].LoanStatus)
	}
	if resp.Applications[1].CustomerStatus != "embryo" {
		t.Fatalf("second customer status = %q, want embryo", resp.Applications[1].CustomerStatus)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestInternalService_GetLoanApplicationBVNRecordForCBA_Success(t *testing.T) {
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
		"",
		"Doe",
		"female",
		"Nigerian",
		"Lagos",
		dob,
		"jane@example.com",
		"08012345678",
		"",
		"Bank Plc",
		"12 Allen Avenue",
		"https://example.com/passport.jpg",
		"",
		"Under bridge",
	)

	mock.ExpectQuery(getLoanApplicationBVNRecordForCBAQueryPattern()).
		WithArgs("user-1", 1).
		WillReturnRows(rows)

	service := NewInternalService(NewInternalRepository(repo.db))

	resp, err := service.GetLoanApplicationBVNRecordForCBA(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetLoanApplicationBVNRecordForCBA returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("GetLoanApplicationBVNRecordForCBA returned nil response")
	}
	if resp.Record.ApplicationRef != "app-ref-123" {
		t.Fatalf("unexpected application ref: %q", resp.Record.ApplicationRef)
	}
	if resp.Record.BVN != "12345678901" {
		t.Fatalf("unexpected bvn: %q", resp.Record.BVN)
	}
	if resp.Record.DateOfBirth != "1995-07-10" {
		t.Fatalf("unexpected date of birth: %q", resp.Record.DateOfBirth)
	}
	if resp.Record.AlternativeMobilePhone != nil {
		t.Fatalf("expected alternative mobile phone to be nil, got %#v", resp.Record.AlternativeMobilePhone)
	}
	if resp.Record.City != nil {
		t.Fatalf("expected city to be nil, got %#v", resp.Record.City)
	}
	if resp.Record.Landmark == nil || *resp.Record.Landmark != "Under bridge" {
		t.Fatalf("unexpected landmark: %#v", resp.Record.Landmark)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
