// Package bvnfacevalidator is the shared, injectable entrypoint for BVN
// face-match validation - mirrors internal/ninvalidator, but there's no
// provider choice to resolve here: BVN-with-face is Prembly-only (Tendar
// exposes no equivalent endpoint), so this just centralizes the
// verification-row lookup, the provider call, and the FaceCheckRecord
// persistence that would otherwise be duplicated at every call site.
package bvnfacevalidator

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"neat_mobile_app_backend/internal/crypto"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/providers/bvn"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Client is satisfied by the Prembly BVN provider client.
type Client interface {
	ValidateBVNWithFace(ctx context.Context, number, image string) (*bvn.PremblyBVNWithFaceResponse, error)
}

// Result is what a successful validation hands back to the caller - just
// enough to reference the persisted FaceCheckRecord.
type Result struct {
	FaceCheckID string
}

type Validator struct {
	db     *gorm.DB
	client Client
	cipher *crypto.FieldCipher
}

func New(db *gorm.DB, client Client, cipher *crypto.FieldCipher) *Validator {
	return &Validator{db: db, client: client, cipher: cipher}
}

// ValidateBVNWithFace confirms verificationID refers to an already-verified
// BVN matching bvnNumber, then runs a live face match against image via
// Prembly, persisting the outcome as a FaceCheckRecord either way.
func (v *Validator) ValidateBVNWithFace(ctx context.Context, verificationID, bvnNumber, image string) (*Result, error) {
	verificationID = strings.TrimSpace(verificationID)
	bvnNumber = strings.TrimSpace(bvnNumber)
	image = strings.TrimSpace(image)

	record, err := v.getValidationRow(ctx, verificationID)
	if err != nil {
		log.Printf("bvnfacevalidator: failed to load verification row id=%s: %v", verificationID, err)
		return nil, appErr.ErrInvalidVerificationID
	}

	if record.VerifiedID == nil {
		log.Printf("bvnfacevalidator: verification row id=%s has no verified id", verificationID)
		return nil, appErr.ErrInvalidVerificationID
	}
	if *record.VerifiedID != bvnNumber {
		log.Printf("bvnfacevalidator: bvn mismatch for verification row id=%s", verificationID)
		return nil, appErr.ErrInvalidBVN
	}

	resp, err := v.client.ValidateBVNWithFace(ctx, bvnNumber, image)
	if err != nil {
		// Only a genuine face comparison result should be reported as a mismatch;
		// a timeout or an unconfigured key must not tell the user their face is wrong.
		log.Printf("bvnfacevalidator: provider call failed: %v", err)
		return nil, translateProviderError(err, appErr.ErrValidatingBVNWithFace)
	}

	faceRecord := &models.FaceCheckRecord{
		ID:                   uuid.NewString(),
		VerificationRecordID: record.ID,
		Provider:             "prembly",
		Matched:              resp.FaceData.Status,
		Confidence:           resp.FaceData.Confidence,
		ResponseCode:         resp.FaceData.ResponseCode,
		ProviderMessage:      resp.FaceData.Message,
	}
	if faceImage := strings.TrimSpace(resp.FaceData.FaceImageProvided); faceImage != "" {
		faceRecord.FaceImageProvided = &faceImage
	}
	if refID := strings.TrimSpace(resp.BillingInfo.ReferenceID); refID != "" {
		faceRecord.ProviderReferenceID = &refID
	}
	if txnID := strings.TrimSpace(resp.BillingInfo.TransactionID); txnID != "" {
		faceRecord.TransactionID = &txnID
	}

	if err := v.createFaceCheckRecord(ctx, faceRecord); err != nil {
		log.Printf("bvnfacevalidator: CreateFaceCheckRecord failed: %v", err)
	}

	if !resp.FaceData.Status {
		return nil, appErr.ErrValidatingBVNWithFace
	}

	return &Result{FaceCheckID: faceRecord.ID}, nil
}

func (v *Validator) getValidationRow(ctx context.Context, verificationID string) (*models.VerificationRecord, error) {
	var record models.VerificationRecord
	err := v.db.WithContext(ctx).Table("wallet_verification_records").
		Select("id, type, verified_name, verified_dob, verified_phone, verified_email, verified_id, verified_gender, verified_marital_status, verified_full_home_address, expires_at").
		Where("id = ? AND status = ? AND expires_at > ?", verificationID, models.VerificationStatusVerified, time.Now().UTC()).
		First(&record).Error
	if err != nil {
		return nil, err
	}

	if record.VerifiedID != nil {
		plain, err := v.cipher.Decrypt(*record.VerifiedID)
		if err != nil {
			return nil, err
		}
		record.VerifiedID = &plain
	}
	return &record, nil
}

func (v *Validator) createFaceCheckRecord(ctx context.Context, record *models.FaceCheckRecord) error {
	return v.db.WithContext(ctx).Create(record).Error
}

// providerInfraCodes/translateProviderError/classifyProviderFailure mirror
// auth.translateProviderError - duplicated rather than imported to keep this
// package free of any internal/modules dependency (importing auth here would
// risk a cycle, since auth already depends on plenty else in this app).
var providerInfraCodes = map[string]struct{}{
	"CLIENT_ERROR":     {},
	"TIMEOUT":          {},
	"NETWORK_ERROR":    {},
	"INVALID_RESPONSE": {},
}

func translateProviderError(err error, notFound error) error {
	if err == nil {
		return nil
	}

	var premblyErr *appErr.PremblyError
	if errors.As(err, &premblyErr) {
		return classifyProviderFailure("prembly", premblyErr.Status, premblyErr.Code, premblyErr.Retryable, notFound, err)
	}

	return err
}

func classifyProviderFailure(provider string, status int, code string, retryable bool, notFound, original error) error {
	if _, infra := providerInfraCodes[code]; infra || retryable {
		log.Printf("identity provider %s unavailable: status=%d code=%s err=%v", provider, status, code, original)
		return appErr.ErrProviderServiceUnavailable
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		log.Printf("identity provider %s rejected our credentials: status=%d code=%s", provider, status, code)
		return appErr.ErrProviderServiceUnavailable
	case status == http.StatusNotFound || status == http.StatusUnprocessableEntity || code == "VALIDATION_ERROR":
		return notFound
	default:
		return original
	}
}
