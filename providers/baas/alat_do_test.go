package baas

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	appErr "neat_mobile_app_backend/internal/errors"
)

// An error reply carrying ALAT's envelope returns the decoded body too.
func TestAlatDo_ErrorReplyKeepsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"data":{"accountGenerationStatus":"Pending","trackingId":"t-1"},"message":"already exist","status":false}`))
	}))
	defer srv.Close()
	a := NewAlat(srv.URL, "key", "sub", "")

	resp, err := alatDo[AlatAccountCreationResponse](context.Background(), a, http.MethodPost, srv.URL+"/x", map[string]string{})
	var alatErr *appErr.AlatError
	if !errors.As(err, &alatErr) || alatErr.Message != "already exist" || alatErr.NoResponse {
		t.Fatalf("err = %v, want ALAT's answer", err)
	}
	if resp == nil || resp.Data == nil || resp.Data.AccountGenerationStatus != "Pending" || resp.Data.TrackingID != "t-1" {
		t.Fatalf("resp = %+v, want the decoded body", resp)
	}
}

// Live ALAT replies send code/statusCode as numbers; the spec shows strings.
func TestAlatEnvelope_NumericOrStringCodes(t *testing.T) {
	for body, want := range map[string]string{
		`{"status":true,"code":0,"statusCode":0,"data":{"accountNumber":"0123456789"}}`:                     "0",
		`{"status":true,"code":"InvalidBvn","statusCode":"Continue","data":{"accountNumber":"0123456789"}}`: "InvalidBvn",
		`{"status":true,"code":null,"data":{"accountNumber":"0123456789"}}`:                                 "",
	} {
		var r AlatAccountDetailsResponse
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if string(r.Code) != want || r.Data == nil || r.Data.AccountNumber != "0123456789" {
			t.Fatalf("%s: code=%q data=%+v", body, r.Code, r.Data)
		}
	}
}

// The debit/credit wallet APIs use {errorMessage, errorMessages, hasError};
// their message must survive a non-2xx reply.
func TestAlatDo_WalletEnvelopeErrorMessage(t *testing.T) {
	const body = `{"result":null,"errorMessage":"Account enquiry failed, confirm that account number is valid","errorMessages":null,"hasError":true}`
	for _, tc := range []struct {
		status     int
		body       string
		wantMsg    string
		noResponse bool
	}{
		{http.StatusBadRequest, body, "Account enquiry failed, confirm that account number is valid", false},
		{http.StatusInternalServerError, body, "Account enquiry failed, confirm that account number is valid", true},
		{http.StatusBadRequest, "not json", "alat request returned an unexpected status", true},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		a := NewAlat(srv.URL, "key", "sub", "")
		_, err := alatDo[AlatAccountNameEnquiryResponse](context.Background(), a, http.MethodGet, srv.URL+"/x", nil)
		srv.Close()
		var alatErr *appErr.AlatError
		if !errors.As(err, &alatErr) || alatErr.Message != tc.wantMsg || alatErr.NoResponse != tc.noResponse {
			t.Fatalf("status %d: err = %v, want message %q noResponse=%t", tc.status, err, tc.wantMsg, tc.noResponse)
		}
	}
}

func TestAlatDo_AccessHeaderOnDebitWalletOnly(t *testing.T) {
	got := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got[r.URL.Path] = r.Header.Get("Access") + "|" + r.Header.Get("x-api-key")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	a := NewAlat(srv.URL, "key", "sub", "")
	for _, p := range []string{"/debit-wallet/api/Shared/ProcessClientTransfer", "/credit-wallet/api/x"} {
		_, _ = alatDo[AlatClientTransferResponse](context.Background(), a, http.MethodPost, srv.URL+p, map[string]string{})
	}
	if got["/debit-wallet/api/Shared/ProcessClientTransfer"] != "key|key" {
		t.Fatalf("debit wallet headers = %q", got["/debit-wallet/api/Shared/ProcessClientTransfer"])
	}
	if got["/credit-wallet/api/x"] != "|key" {
		t.Fatalf("credit wallet headers = %q", got["/credit-wallet/api/x"])
	}
}
