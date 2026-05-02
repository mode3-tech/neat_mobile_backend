package providus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookupWalletByCustomerID_Success(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/wallet/customer" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("customerId"); got != "customer-123" {
			t.Fatalf("unexpected customerId query: %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": true,
			"wallet": map[string]any{
				"id":                    "bea49828-2f82-412a-8a13-f77f919b9ae7",
				"type":                  "STATIC",
				"tier":                  "TIER_1",
				"status":                "ACTIVE",
				"email":                 "ykdda.mnnslk@example.com",
				"customerId":            "customer-123",
				"lastName":              "Mnnslk",
				"firstName":             "Ykdda",
				"bankName":              "Xpresswallet",
				"bankCode":              "100040",
				"createdAt":             "2026-05-01T18:13:30.484Z",
				"updatedAt":             "2026-05-01T18:13:30.484Z",
				"accountName":           "Ykdda Mnnslk",
				"phoneNumber":           "2347065990892",
				"accountNumber":         "4408280400",
				"bookedBalance":         0,
				"availableBalance":      0,
				"accountReference":      "mXZqdJcvhrkSpDO5EbPzu3LeEJlVXZFLUeXn",
				"dailyTransactionLimit": 50000,
			},
		})
	}))
	defer server.Close()

	client := NewProvidus("secret-key", server.URL)
	resp, found, err := client.LookupWalletByCustomerID(context.Background(), "customer-123")
	if err != nil {
		t.Fatalf("LookupWalletByCustomerID returned error: %v", err)
	}
	if !found {
		t.Fatal("expected wallet lookup to be found")
	}
	if resp == nil || resp.Customer == nil || resp.Wallet == nil {
		t.Fatal("expected decoded customer and wallet response")
	}
	if resp.Customer.ID != "customer-123" {
		t.Fatalf("unexpected customer id: %q", resp.Customer.ID)
	}
	if resp.Wallet.AccountNumber != "4408280400" {
		t.Fatalf("unexpected account number: %q", resp.Wallet.AccountNumber)
	}
	if resp.Wallet.WalletType != "STATIC" {
		t.Fatalf("unexpected wallet type: %q", resp.Wallet.WalletType)
	}
	if resp.Customer.Tier != "TIER_1" {
		t.Fatalf("unexpected customer tier: %q", resp.Customer.Tier)
	}
}

func TestLookupWalletByCustomerID_NotFound(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewProvidus("secret-key", server.URL)
	resp, found, err := client.LookupWalletByCustomerID(context.Background(), "missing-customer")
	if err != nil {
		t.Fatalf("LookupWalletByCustomerID returned error: %v", err)
	}
	if found {
		t.Fatal("expected missing wallet lookup to return found=false")
	}
	if resp != nil {
		t.Fatalf("expected nil response for missing customer, got %+v", resp)
	}
}
