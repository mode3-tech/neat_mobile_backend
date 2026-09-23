package tier

import (
	"context"
	"errors"
	"time"

	"neat_mobile_app_backend/internal/crypto"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/helpers"
	"neat_mobile_app_backend/internal/middleware"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repo        *Repository
	ninService  NINService
	userService UserService
	faceService FaceService
	cipher      *crypto.FieldCipher
	auditLogger auditlog.AuditLogger
}

func NewService(repo *Repository, ninService NINService, userService UserService, faceService FaceService, cipher *crypto.FieldCipher, auditLogger auditlog.AuditLogger) *Service {
	return &Service{
		repo:        repo,
		ninService:  ninService,
		userService: userService,
		faceService: faceService,
		cipher:      cipher,
		auditLogger: auditLogger,
	}
}

func (s *Service) logTierAudit(ctx context.Context, action, mobileUserID string, status auditlog.LogStatus, metadata map[string]interface{}) {
	s.auditLogger.CreateAuditLog(ctx, &auditlog.AuditLog{
		ID:           helpers.PrefixID("audit_log"),
		ResourceID:   mobileUserID,
		ResourceType: auditlog.ResourceTypeTierUpgrade,
		ActorType:    "user",
		RequestID:    middleware.GetRequestID(ctx),
		Timestamp:    time.Now().UTC(),
		Status:       status,
		ActorID:      mobileUserID,
		Action:       action,
		IPAddress:    middleware.GetClientIP(ctx),
		Metadata:     metadata,
	})
}

func (s *Service) GetRequirementStatus(ctx context.Context, mobileUserID string) ([]RequirementCheck, error) {
	user, err := s.userService.GetUserDetails(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrUserNotFound
		}
		return nil, err
	}

	latest, err := s.repo.GetLatestSubmissions(ctx, mobileUserID)
	if err != nil {
		return nil, err
	}

	all := []TierRequirement{
		TierRequirementNIN,
		TierRequirementPassportPhotograph,
		TierRequirementUtilityBill,
		TierRequirementAddressVerification,
	}

	results := make([]RequirementCheck, 0, len(all))
	for _, req := range all {
		if req == TierRequirementNIN {
			status := RequirementStatusNotSubmitted
			if user.IsNinVerified {
				status = RequirementStatusApproved
			}
			results = append(results, RequirementCheck{Requirement: req, Status: status})
			continue
		}

		sub, ok := latest[req]
		if !ok {
			results = append(results, RequirementCheck{Requirement: req, Status: RequirementStatusNotSubmitted})
			continue
		}
		results = append(results, RequirementCheck{Requirement: req, Status: RequirementStatus(sub.Status)})
	}
	return results, nil
}

func (s *Service) ValidateNIN(ctx context.Context, mobileUserID string) error {
	user, err := s.userService.GetUserDetails(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appErr.ErrUserNotFound
		}
		return err
	}

	plainNIN, err := s.cipher.Decrypt(user.NIN)
	if err != nil {
		return err
	}

	ninDetails, err := s.ninService.ValidateNIN(ctx, plainNIN)
	if err != nil {
		return err
	}

	middleName := ""
	if user.MiddleName != nil {
		middleName = *user.MiddleName
	}

	userFullName := user.FirstName + " " + middleName + " " + user.LastName
	ninFullName := ninDetails.Data.FirstName + " " + ninDetails.Data.MiddleName + " " + ninDetails.Data.Surname

	if !helpers.DOBsMatch(user.DOB.Format("2006-01-02"), ninDetails.Data.BirthDate) {
		s.logTierAudit(ctx, "TIER_NIN_VALIDATION", mobileUserID, auditlog.StatusFailure, map[string]interface{}{
			"reason_for_failure": "dob mismatch",
		})
		return appErr.ErrNINDOBMismatch
	}

	if !helpers.NamesMatch(userFullName, ninFullName) {
		s.logTierAudit(ctx, "TIER_NIN_VALIDATION", mobileUserID, auditlog.StatusFailure, map[string]interface{}{
			"reason_for_failure": "name mismatch",
		})
		return appErr.ErrNINNameMismatch
	}

	ninHash := crypto.Hash(plainNIN)

	err = s.userService.UpdateUserNIN(ctx, mobileUserID, true, ninHash)
	if err != nil {
		s.logTierAudit(ctx, "TIER_NIN_VALIDATION", mobileUserID, auditlog.StatusFailure, map[string]interface{}{
			"reason_for_failure": "failed to persist nin verification",
		})
		return err
	}

	s.logTierAudit(ctx, "TIER_NIN_VALIDATION", mobileUserID, auditlog.StatusSuccess, nil)
	return nil
}

// ValidateNINWithFace runs a live NIN-with-face match for the user's stored
// NIN/DOB against image, and records the outcome as a passport_photograph
// tier submission - the submitted image doubles as the passport photograph,
// and a match also satisfies the NIN requirement (Prembly/Tendar cross-check
// NIN against BVN on their end, so this covers both).
func (s *Service) ValidateNINWithFace(ctx context.Context, mobileUserID, image string) error {
	user, err := s.userService.GetUserDetails(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appErr.ErrUserNotFound
		}
		return err
	}

	plainNIN, err := s.cipher.Decrypt(user.NIN)
	if err != nil {
		return err
	}

	result, err := s.faceService.ValidateNINWithFace(ctx, plainNIN, user.DOB.Format("2006-01-02"), image)
	if err != nil {
		return err
	}

	status := TierSubmissionStatusRejected
	if result.Matched {
		status = TierSubmissionStatusApproved
	}

	submission := &TierSubmission{
		ID:          uuid.NewString(),
		UserID:      mobileUserID,
		Requirement: TierRequirementPassportPhotograph,
		DocumentURL: image,
		Status:      status,
	}
	if !result.Matched {
		reason := result.Message
		submission.RejectionReason = &reason
	}
	if err := s.repo.CreateSubmission(ctx, submission); err != nil {
		return err
	}

	if !result.Matched {
		s.logTierAudit(ctx, "TIER_NIN_FACE_VALIDATION", mobileUserID, auditlog.StatusFailure, map[string]interface{}{
			"reason_for_failure": result.Message,
		})
		return appErr.ErrValidatingNINWithFace
	}

	ninHash := crypto.Hash(plainNIN)
	if err := s.userService.UpdateUserNIN(ctx, mobileUserID, true, ninHash); err != nil {
		s.logTierAudit(ctx, "TIER_NIN_FACE_VALIDATION", mobileUserID, auditlog.StatusFailure, map[string]interface{}{
			"reason_for_failure": "failed to persist nin verification",
		})
		return err
	}

	s.logTierAudit(ctx, "TIER_NIN_FACE_VALIDATION", mobileUserID, auditlog.StatusSuccess, map[string]interface{}{
		"submission_id": submission.ID,
	})
	return nil
}
