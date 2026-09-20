package auth

import (
	"context"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/models"
	ninpkg "neat_mobile_app_backend/providers/nin"
	"strings"

	"github.com/google/uuid"
)

func (s *Service) ValidateBVNWithFace(ctx context.Context, payload BVNWithFaceValidationRequest) (*bvnWithFaceInfo, error) {
	verificationID := strings.TrimSpace(payload.VerificationID)
	bvn := strings.TrimSpace(payload.BVN)
	image := strings.TrimSpace(payload.Image)

	record, err := s.repo.GetValidationRow(ctx, verificationID)
	if err != nil {
		log.Printf("%s: %s", appErr.ErrInvalidVerificationID, err)
		return nil, appErr.ErrInvalidVerificationID
	}

	if record.VerifiedID == nil {
		log.Printf("%s", appErr.ErrInvalidVerificationID)
		return nil, appErr.ErrInvalidVerificationID
	}

	if *record.VerifiedID != bvn {
		log.Printf("%s", appErr.ErrInvalidBVN)
		return nil, appErr.ErrInvalidBVN
	}

	provider := ProviderPrembly
	if s.resolveProvider(ctx) == ProviderTendar && s.bvnFaceTendar != nil {
		provider = ProviderTendar
	}

	// Idempotency guard: if this verification record already has a successful face
	// check, return it without invoking (and being billed by) the provider again.
	existing, existingErr := s.repo.GetFaceCheckRecordByVerificationID(ctx, record.ID)
	if existingErr == nil && existing != nil && existing.Matched {
		return &bvnWithFaceInfo{faceCheckID: existing.ID}, nil
	}

	var faceResult *bvnFaceValidationResult
	switch provider {
	case ProviderTendar:
		resp, err := s.bvnFaceTendar.ValidateBVNWithFace(ctx, bvn, image)
		if err != nil {
			log.Printf("ValidateBVNWithFace: tendar provider call failed: %v", err)
			return nil, translateProviderError(err, appErr.ErrValidatingBVNWithFace)
		}
		faceResult = &bvnFaceValidationResult{
			Matched:    resp.FaceData.Matched,
			Confidence: resp.FaceData.Confidence,
			Message:    resp.FaceData.Message,
		}
	default:
		if s.prembly == nil {
			log.Printf("ValidateBVNWithFace: prembly provider is not configured")
			return nil, appErr.ErrProviderServiceUnavailable
		}
		resp, err := s.prembly.ValidateBVNWithFace(ctx, bvn, image)
		if err != nil {
			// Only a genuine face comparison result should be reported as a mismatch;
			// a timeout or an unconfigured key must not tell the user their face is wrong.
			log.Printf("ValidateBVNWithFace: prembly provider call failed: %v", err)
			return nil, translateProviderError(err, appErr.ErrValidatingBVNWithFace)
		}
		faceResult = &bvnFaceValidationResult{
			Matched:           resp.FaceData.Status,
			Confidence:        resp.FaceData.Confidence,
			Message:           resp.FaceData.Message,
			ResponseCode:      resp.FaceData.ResponseCode,
			FaceImageProvided: strings.TrimSpace(resp.FaceData.FaceImageProvided),
			ReferenceID:       strings.TrimSpace(resp.BillingInfo.ReferenceID),
			TransactionID:     strings.TrimSpace(resp.BillingInfo.TransactionID),
		}
	}

	faceRecord := &models.FaceCheckRecord{
		ID:                   uuid.NewString(),
		VerificationRecordID: record.ID,
		Provider:             string(provider),
		Matched:              faceResult.Matched,
		Confidence:           faceResult.Confidence,
		ResponseCode:         faceResult.ResponseCode,
		ProviderMessage:      faceResult.Message,
	}
	if v := faceResult.FaceImageProvided; v != "" {
		faceRecord.FaceImageProvided = &v
	}
	if v := faceResult.ReferenceID; v != "" {
		faceRecord.ProviderReferenceID = &v
	}
	if v := faceResult.TransactionID; v != "" {
		faceRecord.TransactionID = &v
	}

	if err := s.repo.CreateFaceCheckRecord(ctx, faceRecord); err != nil {
		log.Printf("ValidateBVNWithFace: CreateFaceCheckRecord failed: %v", err)
		return nil, appErr.ErrFaceCheckRecordFailed
	}

	if !faceResult.Matched {
		return nil, appErr.ErrValidatingBVNWithFace
	}

	return &bvnWithFaceInfo{faceCheckID: faceRecord.ID}, nil
}

func (s *Service) ValidateNINWithFace(ctx context.Context, payload NINWithFaceValidationRequest) (*ninWithFaceInfo, error) {
	verificationID := strings.TrimSpace(payload.VerificationID)
	nin := strings.TrimSpace(payload.NIN)
	image := strings.TrimSpace(payload.Image)

	record, err := s.repo.GetValidationRow(ctx, verificationID)
	if err != nil {
		log.Printf("%s: %s", appErr.ErrInvalidVerificationID, err)
		return nil, appErr.ErrInvalidVerificationID
	}

	if record.VerifiedID == nil {
		log.Printf("%s", appErr.ErrInvalidVerificationID)
		return nil, appErr.ErrInvalidVerificationID
	}

	if *record.VerifiedID != nin {
		log.Printf("%s", appErr.ErrInvalidNIN)
		return nil, appErr.ErrInvalidNIN
	}

	provider := ProviderPrembly
	if s.resolveProvider(ctx) == ProviderTendar && s.ninFaceTendar != nil {
		provider = ProviderTendar
	}

	// Idempotency guard: if this verification record already has a successful face
	// check, return it without invoking (and being billed by) the provider again.
	existing, existingErr := s.repo.GetFaceCheckRecordByVerificationID(ctx, record.ID)
	if existingErr == nil && existing != nil && existing.Matched {
		return &ninWithFaceInfo{faceCheckID: existing.ID}, nil
	}

	var faceResult *ninpkg.NINFaceValidationResult
	switch provider {
	case ProviderTendar:
		resp, err := s.ninFaceTendar.ValidateNINWithFace(ctx, nin, image)
		if err != nil {
			log.Printf("ValidateNINWithFace: tendar provider call failed: %v", err)
			return nil, translateProviderError(err, appErr.ErrValidatingNINWithFace)
		}
		faceResult = &resp.FaceData
	default:
		if record.VerifiedDOB == nil {
			log.Printf("ValidateNINWithFace: NIN record missing DOB id=%s", verificationID)
			return nil, appErr.ErrNINRecordMissingDOB
		}
		if s.ninFace == nil {
			log.Printf("ValidateNINWithFace: nin face provider is not configured")
			return nil, appErr.ErrProviderServiceUnavailable
		}
		resp, err := s.ninFace.ValidateNINWithFace(ctx, image, nin, *record.VerifiedDOB)
		if err != nil {
			log.Printf("ValidateNINWithFace: prembly provider call failed: %v", err)
			return nil, translateProviderError(err, appErr.ErrValidatingNINWithFace)
		}
		faceResult = &ninpkg.NINFaceValidationResult{
			Matched:      resp.FaceData.Status,
			Confidence:   resp.FaceData.Confidence,
			Message:      resp.FaceData.Message,
			ResponseCode: resp.FaceData.ResponseCode,
		}
	}

	faceRecord := &models.FaceCheckRecord{
		ID:                   uuid.NewString(),
		VerificationRecordID: record.ID,
		Provider:             string(provider),
		Matched:              faceResult.Matched,
		Confidence:           faceResult.Confidence,
		ResponseCode:         faceResult.ResponseCode,
		ProviderMessage:      faceResult.Message,
	}

	if err := s.repo.CreateFaceCheckRecord(ctx, faceRecord); err != nil {
		log.Printf("ValidateNINWithFace: CreateFaceCheckRecord failed: %v", err)
		return nil, appErr.ErrFaceCheckRecordFailed
	}

	if !faceResult.Matched {
		return nil, appErr.ErrValidatingNINWithFace
	}

	return &ninWithFaceInfo{faceCheckID: faceRecord.ID}, nil
}
