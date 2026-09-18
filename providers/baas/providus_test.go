package baas

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"neat_mobile_app_backend/internal/modules/auth"
)

func TestGenerateWallet_RequestBodyMatchesProvidusSchema(t *testing.T) {
	t.Helper()

	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/wallet" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("failed to decode captured request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": true})
	}))
	defer server.Close()

	client := NewProvidus("secret-key", server.URL)
	_, err := client.GenerateWallet(context.Background(), &auth.WalletPayload{
		// Fields Providus's documented schema expects.
		BVN:         "12345678901",
		FirstName:   "First",
		LastName:    "User",
		DateOfBirth: "1992-05-16",
		PhoneNumber: "2348020245349",
		Email:       "access2emma@gmail.com",
		Address:     "No 10, Adewale Ajasin University",
		Metadata:    map[string]interface{}{"even-more": "Other data"},
		// Optimus-only fields that must NOT be sent to Providus.
		RequestID:         "should-not-be-sent",
		MothersMaidenName: "should-not-be-sent",
		HouseNo:           "should-not-be-sent",
		ProductId:         "should-not-be-sent",
		Gender:            "should-not-be-sent",
		MaritalStatus:     "should-not-be-sent",
	})
	if err != nil {
		t.Fatalf("GenerateWallet returned error: %v", err)
	}

	wantKeys := map[string]bool{
		"bvn": true, "firstName": true, "lastName": true, "dateOfBirth": true,
		"phoneNumber": true, "email": true, "address": true, "metadata": true,
	}
	for key := range capturedBody {
		if !wantKeys[key] {
			t.Fatalf("unexpected key %q sent to Providus - request body: %+v", key, capturedBody)
		}
	}
	for key := range wantKeys {
		if _, ok := capturedBody[key]; !ok {
			t.Fatalf("expected key %q missing from request body: %+v", key, capturedBody)
		}
	}
}

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

func TestSeedWalletPayload_Deterministic(t *testing.T) {
	payload := &auth.WalletPayload{
		BVN:         "12345678901",
		FirstName:   "Jane",
		LastName:    "Doe",
		DateOfBirth: "1990-01-01",
		PhoneNumber: "2348012345678",
		Email:       "jane.doe@example.com",
		Address:     "1 Lagos Street",
		Metadata:    map[string]interface{}{"customer_id": "user-123"},
	}

	seededA, err := SeedWalletPayload(payload, "dev-wallet-key", false)
	if err != nil {
		t.Fatal(err)
	}
	seededB, err := SeedWalletPayload(payload, "dev-wallet-key", false)
	if err != nil {
		t.Fatal(err)
	}

	if seededA.Email != seededB.Email {
		t.Fatalf("expected deterministic email with same key; got %q and %q", seededA.Email, seededB.Email)
	}
	if seededA.FirstName != seededB.FirstName {
		t.Fatalf("expected deterministic first name with same key; got %q and %q", seededA.FirstName, seededB.FirstName)
	}
	if seededA.LastName != seededB.LastName {
		t.Fatalf("expected deterministic last name with same key; got %q and %q", seededA.LastName, seededB.LastName)
	}
	if seededA.PhoneNumber != seededB.PhoneNumber {
		t.Fatalf("expected deterministic phone number with same key; got %q and %q", seededA.PhoneNumber, seededB.PhoneNumber)
	}
	if !strings.HasPrefix(seededA.PhoneNumber, "234") {
		t.Fatalf("expected phone number to start with 234, got %q", seededA.PhoneNumber)
	}
	if len(seededA.PhoneNumber) != 13 {
		t.Fatalf("expected phone number to be 13 digits, got %d", len(seededA.PhoneNumber))
	}
	if len(seededB.PhoneNumber) != 13 {
		t.Fatalf("expected phone number to be 13 digits, got %d", len(seededB.PhoneNumber))
	}
	if seededA.BVN != seededB.BVN {
		t.Fatalf("expected deterministic BVN with same key; got %q and %q", seededA.BVN, seededB.BVN)
	}
	if len(seededA.BVN) != 11 {
		t.Fatalf("expected BVN to be 11 digits, got %d", len(seededA.BVN))
	}
	if len(seededB.BVN) != 11 {
		t.Fatalf("expected BVN to be 11 digits, got %d", len(seededB.BVN))
	}
	if seededA.Metadata["wallet_generation_seed"] != seededB.Metadata["wallet_generation_seed"] {
		t.Fatalf("expected deterministic seed with same key")
	}
}

func TestSeedWalletPayload_SecretKeyChange(t *testing.T) {
	payload := &auth.WalletPayload{
		BVN:         "12345678901",
		FirstName:   "Jane",
		LastName:    "Doe",
		DateOfBirth: "1990-01-01",
		PhoneNumber: "2348012345678",
		Email:       "jane.doe@example.com",
		Address:     "1 Lagos Street",
		Metadata:    map[string]interface{}{"customer_id": "user-123"},
	}

	seededA, err := SeedWalletPayload(payload, "dev-wallet-key-a", false)
	if err != nil {
		t.Fatal(err)
	}
	seededB, err := SeedWalletPayload(payload, "dev-wallet-key-b", false)
	if err != nil {
		t.Fatal(err)
	}

	if seededA.Email == seededB.Email {
		t.Fatal("expected different seeded email when secret key changes")
	}
	if seededA.FirstName == seededB.FirstName {
		t.Fatal("expected different seeded first name when secret key changes")
	}
	if seededA.LastName == seededB.LastName {
		t.Fatal("expected different seeded last name when secret key changes")
	}
	if seededA.PhoneNumber == seededB.PhoneNumber {
		t.Fatal("expected different seeded phone number when secret key changes")
	}
	if !strings.HasPrefix(seededA.PhoneNumber, "234") || !strings.HasPrefix(seededB.PhoneNumber, "234") {
		t.Fatal("expected seeded phone numbers to start with 234")
	}
	if len(seededA.PhoneNumber) != 13 || len(seededB.PhoneNumber) != 13 {
		t.Fatal("expected seeded phone numbers to be 13 digits")
	}
	if seededA.BVN == seededB.BVN {
		t.Fatal("expected different seeded BVN when secret key changes")
	}
	if len(seededA.BVN) != 11 || len(seededB.BVN) != 11 {
		t.Fatal("expected seeded BVN to be 11 digits")
	}
}
