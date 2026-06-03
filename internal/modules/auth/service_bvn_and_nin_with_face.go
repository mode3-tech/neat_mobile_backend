package auth

import (
	"context"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/models"
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

	resp, err := s.prembly.ValidateBVNWithFace(ctx, bvn, image)
	if err != nil {
		return nil, appErr.ErrValidatingBVNWithFace
	}

	faceRecord := &models.FaceCheckRecord{
		ID:                   uuid.NewString(),
		VerificationRecordID: record.ID,
		Provider:             string(ProviderPrembly),
		Matched:              resp.FaceData.Status,
		Confidence:           resp.FaceData.Confidence,
		ResponseCode:         resp.FaceData.ResponseCode,
		ProviderMessage:      resp.FaceData.Message,
	}
	if v := strings.TrimSpace(resp.FaceData.FaceImageProvided); v != "" {
		faceRecord.FaceImageProvided = &v
	}
	if v := strings.TrimSpace(resp.BillingInfo.ReferenceID); v != "" {
		faceRecord.ProviderReferenceID = &v
	}
	if v := strings.TrimSpace(resp.BillingInfo.TransactionID); v != "" {
		faceRecord.TransactionID = &v
	}

	if err := s.repo.CreateFaceCheckRecord(ctx, faceRecord); err != nil {
		log.Printf("ValidateBVNWithFace: CreateFaceCheckRecord failed: %v", err)
	}

	if !resp.FaceData.Status {
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

	if record.VerifiedDOB == nil {
		log.Printf("ValidateNINWithFace: NIN record missing DOB id=%s", verificationID)
		return nil, appErr.ErrInvalidVerificationID
	}

	resp, err := s.nin.ValidateNINWithFace(ctx, image, nin, *record.VerifiedDOB)
	if err != nil {
		return nil, appErr.ErrValidatingNINWithFace
	}

	faceRecord := &models.FaceCheckRecord{
		ID:                   uuid.NewString(),
		VerificationRecordID: record.ID,
		Provider:             string(ProviderPrembly),
		Matched:              resp.FaceData.Status,
		Confidence:           resp.FaceData.Confidence,
		ResponseCode:         resp.FaceData.ResponseCode,
		ProviderMessage:      resp.FaceData.Message,
	}

	if err := s.repo.CreateFaceCheckRecord(ctx, faceRecord); err != nil {
		log.Printf("ValidateNINWithFace: CreateFaceCheckRecord failed: %v", err)
	}

	if !resp.FaceData.Status {
		return nil, appErr.ErrValidatingNINWithFace
	}

	return &ninWithFaceInfo{faceCheckID: faceRecord.ID}, nil
}
