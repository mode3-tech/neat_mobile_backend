package tier

import (
	"context"
	"errors"

	"neat_mobile_app_backend/internal/crypto"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/helpers"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repo        *Repository
	ninService  NINService
	userService UserService
	faceService FaceService
	cipher      *crypto.FieldCipher
}

func NewService(repo *Repository, ninService NINService, userService UserService, faceService FaceService, cipher *crypto.FieldCipher) *Service {
	return &Service{
		repo:        repo,
		ninService:  ninService,
		userService: userService,
		faceService: faceService,
		cipher:      cipher,
	}
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
		return appErr.ErrNINDOBMismatch
	}

	if !helpers.NamesMatch(userFullName, ninFullName) {
		return appErr.ErrNINNameMismatch
	}

	ninHash := crypto.Hash(plainNIN)

	err = s.userService.UpdateUserNIN(ctx, mobileUserID, true, ninHash)
	if err != nil {
		return err
	}
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
		return appErr.ErrValidatingNINWithFace
	}

	ninHash := crypto.Hash(plainNIN)
	return s.userService.UpdateUserNIN(ctx, mobileUserID, true, ninHash)
}
