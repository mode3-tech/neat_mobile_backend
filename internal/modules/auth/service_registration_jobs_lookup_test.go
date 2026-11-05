package auth

import (
	"testing"
	"time"
)

func TestWalletLookupKey_UsesPhoneForWemaAndCustomerIDOtherwise(t *testing.T) {
	job := &RegistrationJob{MobileUserID: "user-uuid"}
	snapshot := &registrationJobSnapshot{Phone: "2348000000000", WalletPhone: "2348012345678", DOB: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)}

	for _, provider := range []string{"wema", "ALAT"} {
		s := &Service{walletProviderName: provider}
		if got := s.walletLookupKey(job, snapshot); got != "2348012345678" {
			t.Fatalf("%s: expected wallet phone, got %q", provider, got)
		}
	}
	for _, provider := range []string{"optimus", "providus", ""} {
		s := &Service{walletProviderName: provider}
		if got := s.walletLookupKey(job, snapshot); got != "user-uuid" {
			t.Fatalf("%q: expected customer id, got %q", provider, got)
		}
	}
}

// With seeding on, the lookup must use the same decorated phone that
// GenerateWallet was sent.
func TestWalletLookupKey_AppliesSeedLikeGenerate(t *testing.T) {
	job := &RegistrationJob{MobileUserID: "user-uuid"}
	snapshot := &registrationJobSnapshot{WalletPhone: "2348012345678", BVN: "12345678901", WalletEmail: "a@b.com", DOB: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)}
	s := &Service{walletProviderName: "wema", walletPayloadSeedKey: "seed-key"}

	want, err := SeedWalletPayload(&WalletPayload{
		BVN: snapshot.BVN, PhoneNumber: snapshot.WalletPhone, Email: snapshot.WalletEmail, DateOfBirth: "1990-01-01",
		FirstName: "Any", LastName: "Name", Address: "x", // fields that don't affect the seed
	}, "seed-key", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.walletLookupKey(job, snapshot); got != want.PhoneNumber {
		t.Fatalf("expected seeded phone %q, got %q", want.PhoneNumber, got)
	}
}
