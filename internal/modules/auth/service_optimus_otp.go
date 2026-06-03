package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	phoneUtil "neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/models"
	"strings"
	"time"

	"github.com/google/uuid"
)

// VerifyOptimusPhoneOTP validates the OTP with Optimus and, on success, creates a
// phone VerificationRecord in the DB. Returns the phoneVerificationID to be used in
// the RegisterationRequest.
func (s *Service) VerifyOptimusPhoneOTP(ctx context.Context, phone, otpToken, email, referenceID string) (string, error) {
	if s.optimusKYC == nil {
		return "", fmt.Errorf("optimus KYC provider not configured")
	}

	normalizedPhone, err := phoneUtil.NormalizeNigerianNumber(strings.TrimSpace(phone))
	if err != nil {
		log.Printf("VerifyOptimusPhoneOTP: phone normalization failed phone=%q err=%v", phone, err)
		return "", err
	}

	if err := s.optimusKYC.VerifyOTPWithOptimus(ctx, normalizedPhone, strings.TrimSpace(otpToken), strings.TrimSpace(email), strings.TrimSpace(referenceID)); err != nil {
		log.Printf("VerifyOptimusPhoneOTP: provider verification failed: %v", err)
		return "", err
	}

	verificationID := uuid.NewString()
	subjectHashBytes := sha256.Sum256([]byte(normalizedPhone))
	subjectHash := hex.EncodeToString(subjectHashBytes[:])
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	maskedPhone, _ := phoneUtil.MaskPhone(normalizedPhone)

	record := &models.VerificationRecord{
		ID:                     verificationID,
		Type:                   models.VerificationTypePhone,
		Provider:               string(ProviderOptimus),
		Status:                 models.VerificationStatusVerified,
		SubjectHash:            subjectHash,
		SubjectMasked:          &maskedPhone,
		ProviderVerificationID: &referenceID,
		VerifiedPhone:          &normalizedPhone,
		VerifiedAt:             &now,
		ExpiresAt:              &expiresAt,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := s.verification.AddVerification(ctx, record); err != nil {
		log.Printf("VerifyOptimusPhoneOTP: AddVerification failed: %v", err)
		return "", err
	}

	return verificationID, nil
}
