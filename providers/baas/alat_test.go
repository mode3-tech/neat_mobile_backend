package baas

import (
	"context"
	"encoding/json"
	"neat_mobile_app_backend/internal/modules/auth"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestAlat_PostTier2PartnershipAccountWithFace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/create-account-face/api/partnership/tier2-partnershipaccount-withoutOtp-v2" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req AlatTier2PartnershipRequestWithFace
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.BVN != "12345678901" || req.NIN != "12345678901" || req.LiveImageOfFace != "base64image" {
			t.Errorf("unexpected payload: %+v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(AlatResponseModel{
			Status:     true,
			Message:    "Tier 2 account creation enqueued",
			Code:       "Success",
			StatusCode: "OK",
			Data: &AlatStep1Data{
				TrackingID: "test-tracking-face-123",
			},
		})
	}))
	defer server.Close()

	client := NewAlat(server.URL, "test-api-key", "test-sub-key")
	resp, err := client.PostTier2PartnershipAccountWithFace(context.Background(), AlatTier2PartnershipRequestWithFace{
		BVN:             "12345678901",
		NIN:             "12345678901",
		PhoneNumber:     "08012345678",
		EmailAddress:    "test@example.com",
		LiveImageOfFace: "base64image",
		CorrelationID:   "cor-123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Status {
		t.Errorf("expected status true, got false")
	}
	if resp.Data == nil || resp.Data.TrackingID != "test-tracking-face-123" {
		t.Errorf("expected tracking ID 'test-tracking-face-123', got %+v", resp.Data)
	}
}

func TestAlat_GetAddressDropDownList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/create-account-face/api/CustomerInfo/GetDropDownList" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(AlatDropDownListResponse{
			Status:  true,
			Message: "Success",
			Data: &AlatDropDownListData{
				CountryModel: AlatCountryModel{
					CountryList: []AlatCountry{{ID: 1, CountryCode: "NG", CountryName: "Nigeria"}},
				},
			},
		})
	}))
	defer server.Close()

	client := NewAlat(server.URL, "test-api-key", "test-sub-key")
	resp, err := client.GetAddressDropDownList(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Status {
		t.Errorf("expected status true, got false")
	}
	if resp.Data == nil || len(resp.Data.CountryModel.CountryList) != 1 {
		t.Errorf("expected 1 country, got %+v", resp.Data)
	}
}

// An existing account whose PND can't be lifted must surface as an error, so
// the registration job retries instead of handing back an undebitable account.
func TestAlat_GenerateWallet_ExistingAccountPndFailureReturnsError(t *testing.T) {
	var pndCalls, createCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/GetPartnershipAccountDetails"):
			_ = json.NewEncoder(w).Encode(AlatAccountDetailsResponse{
				Status: true,
				Data:   &AlatAccountDetails{AccountNumber: "0123456789", FirstName: "Test", LastName: "User"},
			})
		case strings.HasSuffix(r.URL.Path, "/PartnerDebitRestrictionManagement"):
			pndCalls++
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"statusCode":400,"code":1,"message":"pnd lift failed","errors":[]}`))
		default:
			createCalls++
			_ = json.NewEncoder(w).Encode(AlatResponseModel{Status: true})
		}
	}))
	defer server.Close()

	client := NewAlat(server.URL, "test-api-key", "test-sub-key")
	_, err := client.GenerateWallet(context.Background(), &auth.WalletPayload{PhoneNumber: "2348012345678", BVN: "12345678901"})
	if err == nil || !strings.Contains(err.Error(), "lift pnd failed") {
		t.Fatalf("expected PND failure error, got %v", err)
	}
	if pndCalls != 1 {
		t.Fatalf("expected one PND call, got %d", pndCalls)
	}
	if createCalls != 0 {
		t.Fatalf("existing account must not be re-created, got %d create calls", createCalls)
	}
}

// A transient lookup failure must not lead on to creating a (possibly
// duplicate) account.
func TestAlat_GenerateWallet_TransientLookupErrorDoesNotCreate(t *testing.T) {
	var createCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/GetPartnershipAccountDetails") {
			w.WriteHeader(http.StatusBadGateway) // no ALAT error envelope
			return
		}
		createCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AlatResponseModel{Status: true})
	}))
	defer server.Close()

	client := NewAlat(server.URL, "test-api-key", "test-sub-key")
	if _, err := client.GenerateWallet(context.Background(), &auth.WalletPayload{PhoneNumber: "2348012345678"}); err == nil {
		t.Fatal("expected an error")
	}
	if createCalls != 0 {
		t.Fatalf("expected no create calls, got %d", createCalls)
	}
}
