package wema

import (
	"context"
	"errors"
	"net/http/httptest"
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
