package wema

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	appErr "neat_mobile_app_backend/internal/errors"
	registerv2 "neat_mobile_app_backend/internal/modules/auth/registerv2"
	"neat_mobile_app_backend/providers/baas"
)

const (
	facePhoneLookup = "/create-account-face/api/CustomerAccount/GetPartnershipAccountDetails"
	bvnPhoneLookup  = "/account-creation/api/CustomerAccount/GetPartnershipAccountDetails"
	noAccount       = `{"message":"Account not found","status":false,"code":"Failed","statusCode":"NotFound"}`
)

// Registration uses the face biometric API only; the BVN account creation
// API must not be queried.
func TestAdapter_GetPartnershipAccountDetails_FaceAPIOnly(t *testing.T) {
	f, alat := newFakeALAT(t, map[string]string{
		facePhoneLookup: `{"status":true,"data":{"accountNumber":"0123456789","firstName":"ADA","lastName":"OBI"}}`,
	})
	d, err := New(alat).GetPartnershipAccountDetails(context.Background(), "08030000000")
	if err != nil || d == nil || d.AccountNumber != "0123456789" {
		t.Fatalf("got %+v, %v; want the face account", d, err)
	}
	if f.call(bvnPhoneLookup) != nil {
		t.Fatal("BVN account creation API was queried")
	}
}

func TestAdapter_LookupWallet_NoAccountIsAnAnswer(t *testing.T) {
	_, alat := newFakeALAT(t, map[string]string{facePhoneLookup: noAccount})
	a := New(alat)

	_, err := a.GetPartnershipAccountDetails(context.Background(), "08030000000")
	var alatErr *appErr.AlatError
	if !errors.As(err, &alatErr) || alatErr.NoResponse {
		t.Fatalf("err = %v; want ALAT's own 'no account' answer", err)
	}
	w, found, err := a.LookupWalletByCustomerID(context.Background(), "08030000000")
	if err != nil || found || w != nil {
		t.Fatalf("LookupWalletByCustomerID = %+v, %v, %v; want not found, no error", w, found, err)
	}
}

// When ALAT can't be reached, the lookup must not read as "no account":
// registration would then create a second account.
func TestAdapter_LookupWallet_UnreachableIsNotNoAccount(t *testing.T) {
	srv := httptest.NewServer(nil)
	url := srv.URL
	srv.Close()
	a := New(baas.NewAlat(url, "key", "sub", ""))

	_, err := a.GetPartnershipAccountDetails(context.Background(), "08030000000")
	var alatErr *appErr.AlatError
	if err == nil || (errors.As(err, &alatErr) && !alatErr.NoResponse) {
		t.Fatalf("err = %v; want a no-response error", err)
	}
	if _, found, err := a.LookupWalletByCustomerID(context.Background(), "08030000000"); err == nil || found {
		t.Fatalf("LookupWalletByCustomerID = found %v, err %v; want an error", found, err)
	}
}

func TestAdapter_LiftPnd(t *testing.T) {
	const path = "/account-creation/api/CustomerAccount/PartnerDebitRestrictionManagement"
	f, alat := newFakeALAT(t, map[string]string{path: `{"status":true,"message":"done"}`})
	if err := New(alat).PartnerDebitRestrictionManagement(context.Background(), "LiftPnd", "0123456789"); err != nil {
		t.Fatal(err)
	}
	c := f.call(path)
	if c == nil || c.body["pndType"] != "LiftPnd" || c.body["accountNumber"] != "0123456789" {
		t.Fatalf("PND call = %+v", c)
	}
}

func TestAdapter_EnvelopeStatusFalseIsAnError(t *testing.T) {
	const path = "/account-creation/api/CustomerAccount/PartnerDebitRestrictionManagement"
	_, alat := newFakeALAT(t, map[string]string{path: `{"status":false,"message":"Account is not on PND","code":"Failed"}`})
	err := New(alat).PartnerDebitRestrictionManagement(context.Background(), "LiftPnd", "0123456789")
	var alatErr *appErr.AlatError
	if !errors.As(err, &alatErr) || alatErr.Message != "Account is not on PND" {
		t.Fatalf("err = %v; want ALAT's message", err)
	}
}

// ALAT wants the local 11-digit phone (08012345678); we store 2348012345678.
func TestAdapter_SendsLocalPhoneFormatToALAT(t *testing.T) {
	f, alat := newFakeALAT(t, map[string]string{
		facePhoneLookup: `{"status":true,"data":{"accountNumber":"0123456789"}}`,
		"/create-account-face/api/partnership/tier2-partnershipaccount-withoutOtp-v2": `{"status":true,"data":{"trackingId":"t-1"}}`,
	})
	a := New(alat)
	if _, err := a.GetPartnershipAccountDetails(context.Background(), "2348012345678"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PostTier2PartnershipAccountWithFace(context.Background(), registerv2.AlatTier2FaceRequest{PhoneNumber: "2348012345678"}); err != nil {
		t.Fatal(err)
	}

	var lookupQuery string
	if c := f.call(facePhoneLookup); c != nil {
		lookupQuery = c.query
	}
	if lookupQuery != "phoneNumber=08012345678" {
		t.Errorf("lookup query = %q, want phoneNumber=08012345678", lookupQuery)
	}
	if c := f.call("/create-account-face/api/partnership/tier2-partnershipaccount-withoutOtp-v2"); c == nil || c.body["phoneNumber"] != "08012345678" {
		t.Errorf("tier 2 phoneNumber = %v, want 08012345678", c)
	}
}

func TestAlatPhone(t *testing.T) {
	for in, want := range map[string]string{
		"2348012345678":  "08012345678",
		"+2348012345678": "08012345678",
		"08012345678":    "08012345678",
		"8012345678":     "08012345678",
		"bad":            "bad",
	} {
		if got := alatPhone(in); got != want {
			t.Errorf("alatPhone(%q) = %q, want %q", in, got, want)
		}
	}
}

// A retry for details ALAT is already generating an account for comes back
// as HTTP 400 / status false with data.accountGenerationStatus "Pending":
// reported as ErrAlatAccountGenerationPending, not a creation failure.
func TestAdapter_Tier2AlreadyPendingAtALAT(t *testing.T) {
	const body = `{"data":{"accountGenerationStatus":"Pending","trackingId":"00000000-0000-0000-0000-000000000000"},` +
		`"message":"00000000 0000000 || example@gmail.com || 080123456789 provided already exist for this channel.",` +
		`"status":false,"code":0,"statusCode":0}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	id, err := New(baas.NewAlat(srv.URL, "key", "sub", "")).PostTier2PartnershipAccountWithFace(context.Background(), registerv2.AlatTier2FaceRequest{PhoneNumber: "2348012345678"})
	if !errors.Is(err, registerv2.ErrAlatAccountGenerationPending) {
		t.Fatalf("err = %v, want ErrAlatAccountGenerationPending", err)
	}
	if id != "" {
		t.Errorf("tracking id = %q, want the all-zero placeholder dropped", id)
	}
}

// Any other refusal is still a failure.
func TestAdapter_Tier2RejectedIsStillAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Hi, please provide a valid phone number","status":false,"code":0,"statusCode":0}`))
	}))
	defer srv.Close()

	_, err := New(baas.NewAlat(srv.URL, "key", "sub", "")).PostTier2PartnershipAccountWithFace(context.Background(), registerv2.AlatTier2FaceRequest{})
	var alatErr *appErr.AlatError
	if errors.Is(err, registerv2.ErrAlatAccountGenerationPending) || !errors.As(err, &alatErr) || alatErr.Message != "Hi, please provide a valid phone number" {
		t.Fatalf("err = %v, want ALAT's rejection", err)
	}
}

// The tier 2 payload is logged as sent, so it can be shared with Wema; only
// the selfie is summarised.
func TestTier2Payload_Logged(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	const image = "/9j/4AAQSkZJRgABAQAAAQABAAD-very-long-selfie-base64-payload"
	_, alat := newFakeALAT(t, map[string]string{
		"/create-account-face/api/partnership/tier2-partnershipaccount-withoutOtp-v2": `{"status":true,"message":"ok","data":{"accountGenerationStatus":"Pending","trackingId":"track-123"}}`,
	})
	if _, err := New(alat).PostTier2PartnershipAccountWithFace(context.Background(), registerv2.AlatTier2FaceRequest{
		BVN: "12345678901", NIN: "10987654321", PhoneNumber: "2348012345678", EmailAddress: "adebayo@example.com",
		LiveImageOfFace: image, CorrelationID: "corr-1",
	}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, want := range []string{
		`op=tier2_create request correlation_id=corr-1`,
		`"bvn":"12345678901"`, `"nin":"10987654321"`,
		`"phoneNumber":"08012345678"`, `"emailAddress":"adebayo@example.com"`,
		`data_uri=false`, `chars`,
		`op=tier2_create response correlation_id=corr-1`,
		`account_generation_status="Pending"`, `tracking_id="track-123"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "very-long-selfie") {
		t.Error("selfie must stay summarised")
	}
}
