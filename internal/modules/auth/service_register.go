package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/authchecker"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/internal/timeutil"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const registrationWalletDefaultAddress = "Address unavailable"

func (s *Service) Register(ctx context.Context, req RegisterationRequest, ip string) (*RegistrationJobResponse, error) {
	if s.tx == nil {
		return nil, errors.New("transaction manager not configured")
	}

	phoneRow, err := s.repo.GetValidationRow(ctx, req.PhoneVerificationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrPhoneNotFound
		}
		return nil, err
	}

	normalizedPhone, err := phone.NormalizeNigerianNumber(strings.TrimSpace(*phoneRow.VerifiedPhone))
	if err != nil {
		return nil, err
	}

	idempotencyKey, err := registrationIdempotencyKey(req, normalizedPhone)
	if err != nil {
		return nil, err
	}

	var job *RegistrationJob
	var claimToken string

	err = s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		authRepo := NewRespository(txDB)

		existingJob, err := authRepo.GetRegistrationJobByIdempotencyKey(ctx, idempotencyKey)
		switch {
		case err == nil:
			if existingJob.Status == RegistrationJobStatusFailed {
				if requeueErr := authRepo.RequeueRegistrationJob(ctx, existingJob.ID); requeueErr != nil {
					return requeueErr
				}
				existingJob.Status = RegistrationJobStatusPending
				existingJob.LastError = nil
			}

			if existingJob.SessionClaimedAt == nil {
				token, tokenHash, claimExpiresAt, tokenErr := newRegistrationClaimToken(time.Now().UTC())
				if tokenErr != nil {
					return tokenErr
				}
				if claimErr := authRepo.SetRegistrationJobClaimToken(ctx, existingJob.ID, tokenHash, claimExpiresAt); claimErr != nil {
					return claimErr
				}

				claimToken = token
				existingJob.SessionClaimTokenHash = &tokenHash
				existingJob.SessionClaimExpiresAt = &claimExpiresAt
			}

			job = existingJob
			return nil
		case !errors.Is(err, gorm.ErrRecordNotFound):
			log.Printf("error checking for existing registration job with idempotency key %s: %v", idempotencyKey, err)
			return err
		}

		openJob, err := authRepo.GetOpenRegistrationJobByPhone(ctx, normalizedPhone)
		switch {
		case err == nil && openJob != nil:
			return appErr.ErrRegistrationAlreadyInProgress
		case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}

		mobileUserID := uuid.NewString()
		internalWalletID := uuid.NewString()
		requestID := uuid.NewString()

		snapshot, buildErr := s.buildRegistrationSnapshot(ctx, authRepo, req, normalizedPhone, mobileUserID, ip, requestID)
		if buildErr != nil {
			return buildErr
		}

		snapshotJSON, buildErr := json.Marshal(snapshot)
		if buildErr != nil {
			return buildErr
		}

		token, tokenHash, claimExpiresAt, tokenErr := newRegistrationClaimToken(time.Now().UTC())
		if tokenErr != nil {
			return tokenErr
		}
		claimToken = token

		job = &RegistrationJob{
			ID:                    uuid.NewString(),
			IdempotencyKey:        idempotencyKey,
			MobileUserID:          mobileUserID,
			InternalWalletID:      internalWalletID,
			Phone:                 normalizedPhone,
			Status:                RegistrationJobStatusPending,
			SnapshotJSON:          string(snapshotJSON),
			SessionClaimTokenHash: &tokenHash,
			SessionClaimExpiresAt: &claimExpiresAt,
		}

		return authRepo.CreateRegistrationJob(ctx, job)
	})
	if err != nil {
		return nil, err
	}

	if job != nil && job.Status != RegistrationJobStatusCompleted {
		s.kickRegistrationProcessing()
	}

	resp := registrationJobResponse(job)
	if resp != nil && strings.TrimSpace(claimToken) != "" {
		resp.ClaimToken = &claimToken
		if job != nil && job.SessionClaimExpiresAt != nil {
			resp.ClaimExpiresAt = job.SessionClaimExpiresAt
		}
	}

	return resp, nil
}

func (s *Service) buildRegistrationSnapshot(ctx context.Context, repo *Repository, req RegisterationRequest, normalizedPhone, mobileUserID, ip, requestID string) (*registrationJobSnapshot, error) {
	phoneRecord, err := repo.GetValidationRow(ctx, req.PhoneVerificationID)
	if err != nil {
		return nil, appErr.ErrPhoneNotFound
	}

	if phoneRecord.VerifiedPhone == nil || *phoneRecord.VerifiedPhone != normalizedPhone {
		return nil, appErr.ErrPhoneMismatch
	}

	existingUser, err := repo.GetUserByPhone(ctx, normalizedPhone)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existingUser != nil {
		return nil, appErr.ErrUserExists
	}

	trimmedEmail := strings.TrimSpace(req.Email)
	if trimmedEmail != "" {
		existingByEmail, emailErr := repo.GetUserByEmail(ctx, trimmedEmail)
		if emailErr != nil && !errors.Is(emailErr, gorm.ErrRecordNotFound) {
			return nil, emailErr
		}
		if existingByEmail != nil {
			return nil, appErr.ErrUserExists
		}
	}

	bvnRecord, err := repo.GetValidationRow(ctx, req.BVNVerificationID)
	if err != nil || bvnRecord.VerifiedName == nil || bvnRecord.VerifiedDOB == nil || bvnRecord.VerifiedID == nil {
		return nil, appErr.ErrBVNNotFound
	}

	faceCheck, err := repo.GetFaceCheckRecord(ctx, req.BVNWithFaceVerificationID)
	if err != nil || !faceCheck.Matched || faceCheck.VerificationRecordID != bvnRecord.ID {
		return nil, appErr.ErrBVNWithFaceVerificationNotFound
	}

	ninRecord, err := repo.GetValidationRow(ctx, req.NINVerificationID)
	if err != nil || ninRecord.VerifiedName == nil || ninRecord.VerifiedDOB == nil || ninRecord.VerifiedID == nil {
		return nil, appErr.ErrNINNotFound
	}

	ninFaceCheck, err := repo.GetFaceCheckRecord(ctx, req.NINWithFaceVerificationID)
	if err != nil || !ninFaceCheck.Matched || ninFaceCheck.VerificationRecordID != ninRecord.ID {
		return nil, appErr.ErrNINWithFaceVerificationNotFound
	}

	var isEmailVerified bool
	var emailRecordUsedID string
	if trimmedEmail != "" {
		emailRecord, emailErr := repo.GetValidationRow(ctx, req.EmailVerificationID)
		if emailErr != nil || emailRecord.VerifiedName == nil || emailRecord.VerifiedDOB == nil {
			return nil, appErr.ErrEmailNotFound
		}

		if emailRecord.VerifiedName != phoneRecord.VerifiedName || emailRecord.VerifiedDOB != phoneRecord.VerifiedDOB {
			return nil, appErr.ErrEmailPhoneMismatch
		}

		isEmailVerified = true
		emailRecordUsedID = emailRecord.ID
	}

	bvnName := strings.ToLower(strings.Join(strings.Fields(*bvnRecord.VerifiedName), " "))
	ninName := strings.ToLower(strings.Join(strings.Fields(*ninRecord.VerifiedName), " "))
	if bvnName != ninName || SerializeDOB(*bvnRecord.VerifiedDOB) != SerializeDOB(*ninRecord.VerifiedDOB) {
		return nil, appErr.ErrNINAndBVNMismatch
	}

	if req.Password != req.ConfirmPassword {
		return nil, appErr.ErrPasswordMismatch
	}
	if err = authchecker.ValidatePassword(req.Password); err != nil {
		log.Printf("invalid password: %v", err)
		return nil, errors.New(err.Error())
	}

	if req.TransactionPin != req.ConfirmTransactionPin {
		return nil, appErr.ErrTransactionPinMismatch
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	pinHash, err := HashPassword(req.TransactionPin)
	if err != nil {
		return nil, err
	}

	dob, err := timeutil.ParseDOB(*ninRecord.VerifiedDOB)
	if err != nil {
		return nil, errors.New(err.Error())
	}

	firstName, middleName, lastName := SplitFullName(*bvnRecord.VerifiedName)

	if err := repo.MarkValidationRecordUsed(ctx, phoneRecord.ID); err != nil {
		log.Printf("failed to mark phone verification record as used")
		return nil, errors.New("failed to mark phone verification record as used")
	}
	if err := repo.MarkValidationRecordUsed(ctx, bvnRecord.ID); err != nil {
		log.Printf("failed to mark bvn verification record as used")
		return nil, errors.New("failed to mark bvn verification record as used")
	}
	if err := repo.MarkValidationRecordUsed(ctx, ninRecord.ID); err != nil {
		log.Printf("failed to mark nin verification record as used")
		return nil, errors.New("failed to mark nin verification record as used")
	}
	if emailRecordUsedID != "" {
		if err := repo.MarkValidationRecordUsed(ctx, emailRecordUsedID); err != nil {
			log.Printf("failed to mark email verification record as used")
			return nil, errors.New("failed to mark email verification record as used")
		}
	}

	address := registrationWalletDefaultAddress
	if bvnRecord.VerifiedFullHomeAddress != nil {
		if v := strings.TrimSpace(*bvnRecord.VerifiedFullHomeAddress); v != "" {
			address = v
		}
	}
	houseNo := extractHouseNumber(address)

	gender := ""
	if bvnRecord.VerifiedGender != nil {
		gender = *bvnRecord.VerifiedGender
	}
	maritalStatus := ""
	if bvnRecord.VerifiedMaritalStatus != nil {
		maritalStatus = *bvnRecord.VerifiedMaritalStatus
	}

	return &registrationJobSnapshot{
		Phone:               normalizedPhone,
		Email:               trimmedEmail,
		PasswordHash:        passwordHash,
		PinHash:             pinHash,
		RequestID:           requestID,
		FirstName:           firstName,
		MiddleName:          strings.TrimSpace(middleName),
		LastName:            lastName,
		BVN:                 *bvnRecord.VerifiedID,
		NIN:                 *ninRecord.VerifiedID,
		DOB:                 dob,
		IsEmailVerified:     isEmailVerified,
		IsPhoneVerified:     true,
		IsBvnVerified:       true,
		IsNinVerified:       true,
		IsBiometricsEnabled: *req.IsBiometricsEnabled,
		Device: DeviceRegisteration{
			DeviceID:    strings.TrimSpace(req.Device.DeviceID),
			PublicKey:   strings.TrimSpace(req.Device.PublicKey),
			DeviceName:  strings.TrimSpace(req.Device.DeviceName),
			DeviceModel: strings.TrimSpace(req.Device.DeviceModel),
			OS:          strings.TrimSpace(req.Device.OS),
			OSVersion:   strings.TrimSpace(req.Device.OSVersion),
			AppVersion:  strings.TrimSpace(req.Device.AppVersion),
		},
		IP:                strings.TrimSpace(ip),
		WalletEmail:       walletRegistrationEmail(trimmedEmail, mobileUserID),
		WalletAddress:     address,
		HouseNo:           houseNo,
		MothersMaidenName: strings.TrimSpace(req.MothersMaidenName),
		Gender:            gender,
		MaritalStatus:     maritalStatus,
		ProductID:         s.productID,
	}, nil
}

func registrationIdempotencyKey(req RegisterationRequest, normalizedPhone string) (string, error) {
	payload := registrationIdempotencyPayload{
		PhoneNumber:               normalizedPhone,
		Email:                     strings.ToLower(strings.TrimSpace(req.Email)),
		BVNVerificationID:         strings.TrimSpace(req.BVNVerificationID),
		BVNWithFaceVerificationID: strings.TrimSpace(req.BVNWithFaceVerificationID),
		NINVerificationID:         strings.TrimSpace(req.NINVerificationID),
		NINWithFaceVerificationID: strings.TrimSpace(req.NINWithFaceVerificationID),
		PhoneVerificationID:       strings.TrimSpace(req.PhoneVerificationID),
		EmailVerificationID:       strings.TrimSpace(req.EmailVerificationID),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// extractHouseNumber parses the numeric house number out of a full address string.
// It splits on the first comma to get the street segment (e.g. "15 Akin Close"),
// then scans each whitespace-delimited token for the first one that is a pure integer.
// Falls back to "1" when no numeric token is found.
func extractHouseNumber(address string) string {
	segment := strings.Split(address, ",")[0]
	for _, token := range strings.Fields(segment) {
		if _, err := strconv.Atoi(token); err == nil {
			return token
		}
	}
	return "1"
}

func walletRegistrationEmail(email, mobileUserID string) string {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail != "" {
		return trimmedEmail
	}

	return fmt.Sprintf("%s@example.com", strings.TrimSpace(mobileUserID))
}
