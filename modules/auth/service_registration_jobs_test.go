package auth

import (
	"testing"
	"time"
)

func TestRegistrationJobCanClaimAt(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	tokenHash := "hashed-claim"
	expiresAt := now.Add(15 * time.Minute)

	job := &RegistrationJob{
		Status:                RegistrationJobStatusCompleted,
		SessionClaimTokenHash: &tokenHash,
		SessionClaimExpiresAt: &expiresAt,
	}

	if !registrationJobCanClaimAt(job, now) {
		t.Fatal("expected completed unclaimed job to be claimable")
	}

	claimedAt := now
	job.SessionClaimedAt = &claimedAt
	if registrationJobCanClaimAt(job, now) {
		t.Fatal("expected claimed job to be unavailable")
	}

	job.SessionClaimedAt = nil
	expiredAt := now.Add(-time.Minute)
	job.SessionClaimExpiresAt = &expiredAt
	if registrationJobCanClaimAt(job, now) {
		t.Fatal("expected expired claim to be unavailable")
	}
}

func TestRegistrationJobResponseCompletedClaimable(t *testing.T) {
	tokenHash := "hashed-claim"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	resp := registrationJobResponse(&RegistrationJob{
		ID:                    "job-1",
		Status:                RegistrationJobStatusCompleted,
		SessionClaimTokenHash: &tokenHash,
		SessionClaimExpiresAt: &expiresAt,
	})
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if !resp.CanLogin {
		t.Fatal("expected completed job to allow login")
	}
	if !resp.CanClaimSession {
		t.Fatal("expected completed job to allow claim")
	}
	if resp.Message != "registration completed successfully, claim session to continue" {
		t.Fatalf("unexpected message: %q", resp.Message)
	}
	if resp.ClaimExpiresAt == nil || !resp.ClaimExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected claim expiry: got %v want %v", resp.ClaimExpiresAt, expiresAt)
	}
}

func TestRegistrationJobResponseCompletedClaimedFallsBackToLogin(t *testing.T) {
	now := time.Now().UTC()
	tokenHash := "hashed-claim"
	expiresAt := now.Add(15 * time.Minute)

	resp := registrationJobResponse(&RegistrationJob{
		ID:                    "job-2",
		Status:                RegistrationJobStatusCompleted,
		SessionClaimTokenHash: &tokenHash,
		SessionClaimExpiresAt: &expiresAt,
		SessionClaimedAt:      &now,
	})
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if !resp.CanLogin {
		t.Fatal("expected completed job to allow login")
	}
	if resp.CanClaimSession {
		t.Fatal("expected claimed job to disallow session claim")
	}
	if resp.Message != "registration completed successfully, login to continue" {
		t.Fatalf("unexpected message: %q", resp.Message)
	}
	if resp.ClaimExpiresAt != nil {
		t.Fatalf("expected no claim expiry after claim, got %v", resp.ClaimExpiresAt)
	}
}
