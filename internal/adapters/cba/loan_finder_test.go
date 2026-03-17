package cba

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderClient_GetCustomerLoans(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if got := r.URL.Path; got != "/internal/customers/123/loans" {
			t.Fatalf("path = %s, want /internal/customers/123/loans", got)
		}
		if got := r.Header.Get("X-Internal-API-Key"); got != "secret-key" {
			t.Fatalf("X-Internal-API-Key = %s, want secret-key", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": "successful",
			"message": "Customer loans fetched successfully",
			"data": [
				{
					"loan_id": "1",
					"loan_number": "LN-0001",
					"principal_amount": 500000,
					"disbursed_amount": 500000,
					"outstanding_balance": 320000,
					"status": "active",
					"next_due_date": "2026-04-01",
					"next_due_amount": 85000
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewProviderClient(server.URL, "secret-key")

	loans, err := client.GetCustomerLoans(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetCustomerLoans returned error: %v", err)
	}
	if len(loans) != 1 {
		t.Fatalf("loan count = %d, want 1", len(loans))
	}
	if loans[0].LoanID != "1" {
		t.Fatalf("LoanID = %s, want 1", loans[0].LoanID)
	}
	if loans[0].Status != "active" {
		t.Fatalf("Status = %s, want active", loans[0].Status)
	}
}

func TestProviderClient_GetLoanDetail(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if got := r.URL.Path; got != "/internal/loans/55" {
			t.Fatalf("path = %s, want /internal/loans/55", got)
		}
		if got := r.Header.Get("X-Internal-API-Key"); got != "secret-key" {
			t.Fatalf("X-Internal-API-Key = %s, want secret-key", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": "successful",
			"message": "Loan details fetched successfully",
			"data": {
				"loan_id": "55",
				"loan_number": "LN-0055",
				"principal_amount": 500000,
				"disbursed_amount": 500000,
				"outstanding_balance": 120000,
				"accrued_interest": 20000,
				"status": "defaulted",
				"next_due_date": "2026-04-01",
				"next_due_amount": 85000
			}
		}`))
	}))
	defer server.Close()

	client := NewProviderClient(server.URL, "secret-key")

	loan, err := client.GetLoanDetail(context.Background(), "55")
	if err != nil {
		t.Fatalf("GetLoanDetail returned error: %v", err)
	}
	if loan == nil {
		t.Fatal("GetLoanDetail returned nil loan")
	}
	if loan.LoanID != "55" {
		t.Fatalf("LoanID = %s, want 55", loan.LoanID)
	}
	if loan.Status != "defaulted" {
		t.Fatalf("Status = %s, want defaulted", loan.Status)
	}
}
