package main

import (
	"os"
	"path/filepath"
	"testing"

	"neat_mobile_app_backend/modules/loanproduct"
)

func TestLoadAndValidateMapsLoanTermValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "biz.json")
	body := `{
  "code": "BUSINESS-WK",
  "name": "Product Loan Business",
  "description": "Short tenor salary-backed product",
  "min_loan_amount": 50000,
  "max_loan_amount": 5000000,
  "interest_rate_bps": 24,
  "repayment_frequency": "weekly",
  "loan_term_value": 24,
  "grace_period_days": 3,
  "late_penalty_bps": 250,
  "allows_concurrent_loans": false,
  "is_active": true
}`

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	row, err := loadAndValidate(path)
	if err != nil {
		t.Fatalf("loadAndValidate returned error: %v", err)
	}

	if row.LoanTermValue != 24 {
		t.Fatalf("LoanTermValue = %d, want 24", row.LoanTermValue)
	}
}

func TestProductUpsertAssignmentsIncludesLoanTermValue(t *testing.T) {
	t.Parallel()

	assignments := productUpsertAssignments(loanproduct.LoanProduct{
		Name:                  "Product Loan Business",
		Description:           "Short tenor salary-backed product",
		MinLoanAmount:         50000,
		MaxLoanAmount:         5000000,
		InterestRateBPS:       24,
		RepaymentFrequency:    loanproduct.LoanFrequencyWeekly,
		LoanTermValue:         24,
		GracePeriodDays:       3,
		LatePenaltyBPS:        250,
		AllowsConcurrentLoans: false,
		IsActive:              true,
	})

	got, ok := assignments["loan_term_value"]
	if !ok {
		t.Fatal("loan_term_value missing from upsert assignments")
	}

	if got != 24 {
		t.Fatalf("loan_term_value = %v, want 24", got)
	}
}
