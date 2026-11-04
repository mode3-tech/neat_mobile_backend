package baas

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAlat_PostPartnershipAccountCreationWithBvn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("expected x-api-key 'test-api-key', got '%s'", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("Ocp-Apim-Subscription-Key") != "test-sub-key" {
			t.Errorf("expected Ocp-Apim-Subscription-Key 'test-sub-key', got '%s'", r.Header.Get("Ocp-Apim-Subscription-Key"))
		}
		if r.URL.Path != "/api/CustomerAccount/PostPartnershipAccountCreationWithBvn" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req AlatPartnershipRequestWithBvn
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.BVN != "12345678901" {
			t.Errorf("expected bvn '12345678901', got '%s'", req.BVN)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(AlatResponseModel{
			Status:     true,
			Message:    "OTP sent successfully",
			Code:       "Success",
			StatusCode: "OK",
		})
	}))
	defer server.Close()

	client := NewAlat(server.URL, "test-api-key", "test-sub-key")
	resp, err := client.PostPartnershipAccountCreationWithBvn(context.Background(), AlatPartnershipRequestWithBvn{
		PhoneNumber: "08012345678",
		Email:       "test@example.com",
		BVN:         "12345678901",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Status {
		t.Errorf("expected status true, got false")
	}
}

func TestAlat_ValidateBVNandEnqueueAccountCreation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/CustomerAccount/ValidateBVNandEnqueueAccountCreation" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req AlatValidateBvnRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.OTP != "123456" {
			t.Errorf("expected otp '123456', got '%s'", req.OTP)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(AlatResponseModel{
			Status:     true,
			Message:    "Account creation enqueued successfully",
			Code:       "Success",
			StatusCode: "OK",
		})
	}))
	defer server.Close()

	client := NewAlat(server.URL, "test-api-key", "test-sub-key")
	resp, err := client.ValidateBVNandEnqueueAccountCreation(context.Background(), AlatValidateBvnRequest{
		PhoneNumber: "08012345678",
		OTP:         "123456",
		TrackingID:  "c6d2d46e-1d6e-4c7c-9ef1-18e809312345",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Status {
		t.Errorf("expected status true, got false")
	}
}

func TestAlat_GetPartnershipAccountDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/CustomerAccount/GetPartnershipAccountDetails" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("phoneNumber") != "08012345678" {
			t.Errorf("expected phoneNumber '08012345678', got '%s'", r.URL.Query().Get("phoneNumber"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(AlatAccountDetailsResponse{
			Status:     true,
			Message:    "Account details retrieved",
			Code:       "Success",
			StatusCode: "OK",
			Data: &AlatAccountDetails{
				AccountNumber: "0123456789",
				FirstName:     "John",
				LastName:      "Doe",
				Email:         "test@example.com",
				PhoneNumber:   "08012345678",
			},
		})
	}))
	defer server.Close()

	client := NewAlat(server.URL, "test-api-key", "test-sub-key")
	resp, err := client.GetPartnershipAccountDetails(context.Background(), "08012345678")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data == nil || resp.Data.AccountNumber != "0123456789" {
		t.Errorf("expected account number '0123456789', got '%v'", resp.Data)
	}
}

func TestAlat_PartnerDebitRestrictionManagement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/CustomerAccount/PartnerDebitRestrictionManagement" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req AlatPndRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.PndType != "LiftPnd" || req.AccountNumber != "0123456789" {
			t.Errorf("unexpected payload: %+v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(AlatResponseModel{
			Status:     true,
			Message:    "Debit restriction lifted successfully",
			Code:       "Success",
			StatusCode: "OK",
		})
	}))
	defer server.Close()

	client := NewAlat(server.URL, "test-api-key", "test-sub-key")
	resp, err := client.PartnerDebitRestrictionManagement(context.Background(), AlatPndRequest{
		PndType:       "LiftPnd",
		AccountNumber: "0123456789",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Status {
		t.Errorf("expected status true, got false")
	}
}

