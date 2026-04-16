package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/auth/verification"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

func (s *Service) ValidateNIN(ctx context.Context, bvnVerificationID, nin string) (*ninInfo, error) {
	if nin == "" || len(nin) < 11 || len(nin) > 11 {
		return nil, errors.New("invalid nin")
	}

	resp, err := s.nin.ValidateNIN(ctx, nin)
	if err != nil {
		return nil, err
	}

	firstName := TitleCase(resp.Data.FirstName)
	middleName := TitleCase(resp.Data.MiddleName)
	lastName := TitleCase(resp.Data.Surname)
	fullName := fmt.Sprintf("%s %s %s", firstName, middleName, lastName)

	row, err := s.repo.GetValidationRow(ctx, bvnVerificationID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("bvn verification not found")
		}
		return nil, err
	}

	err = s.repo.MarkValidationRecordUsed(ctx, row.ID)
	if err != nil {
		return nil, errors.New("failed to mark nin verification record as used")
	}

	_, err = compareBVNAndNinDetails(*row.VerifiedName, SerializeDOB(strings.TrimSpace(*row.VerifiedDOB)), fullName, SerializeDOB(strings.TrimSpace(resp.Data.BirthDate)))

	if err != nil {
		return nil, errors.New(err.Error())
	}

	verificationID := uuid.NewString()
	subjectHashBytes := sha256.Sum256([]byte(strings.TrimSpace(nin)))
	subjectHash := hex.EncodeToString(subjectHashBytes[:])
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	maskedNIN := MaskSub(nin)
	normalizedPhoneNumber, err := NormalizeNigerianNumber(resp.Data.TelephoneNo)
	if err != nil {
		return nil, err
	}

	record := &models.VerificationRecord{
		ID:            verificationID,
		Type:          models.VerificationTypeNIN,
		Status:        models.VerificationStatusVerified,
		Provider:      string(ProviderPrembly),
		SubjectHash:   subjectHash,
		SubjectMasked: &maskedNIN,
		ExpiresAt:     &expiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
		VerifiedID:    &nin,
		VerifiedName:  &fullName,
		VerifiedPhone: &normalizedPhoneNumber,
		VerifiedDOB:   &resp.Data.BirthDate,
		VerifiedEmail: &resp.Data.Email,
	}

	if fullName == "" || strings.TrimSpace(resp.Data.BirthDate) == "" || strings.TrimSpace(resp.Data.TelephoneNo) == "" {
		return nil, errors.New("invalid nin number")
	}

	record.VerifiedName = &fullName
	dob := strings.TrimSpace(resp.Data.BirthDate)
	record.VerifiedDOB = &dob
	phone := strings.TrimSpace(resp.Data.TelephoneNo)
	record.VerifiedPhone = &phone

	if err := s.verification.AddVerification(ctx, record); err != nil {
		return nil, err
	}

	return &ninInfo{
		name:           fullName,
		dob:            dob,
		phone:          phone,
		verificationID: verificationID,
	}, nil
}

func (s *Service) ValidateBVN(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.providerSource == nil {
		return s.ValidateBVNWithTendar(ctx, bvn)
	}

	provider, err := s.providerSource.GetCurrentProvider(ctx)
	if err != nil {
		log.Printf("failed to resolve bvn provider from source; forcing tendar: %v", err)
		return s.ValidateBVNWithTendar(ctx, bvn)
	}

	if provider != ProviderTendar {
		log.Printf("provider source selected %q; forcing tendar-only validation", provider)
	}
	return s.ValidateBVNWithTendar(ctx, bvn)
}

func (s *Service) ValidateBVNWithTendar(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.tender == nil {
		log.Printf("tendar validator is not configured")
		return nil, errors.New("tendar validator is not configured")
	}

	if bvn == "" {
		log.Printf("bvn is required")
		return nil, errors.New("bvn is required")
	}

	if len(bvn) != 11 {
		log.Printf("invalid bvn number")
		return nil, errors.New("invalid bvn number")
	}

	bvnDetails, err := s.tender.ValidateBVNWithTendar(ctx, bvn)
	if err != nil {
		return nil, err
	}
	if bvnDetails == nil {
		log.Printf("invalid bvn number")
		return nil, errors.New("invalid bvn number")
	}

	caser := cases.Title(language.English)

	//Convert the names to titlecase
	fullName := fmt.Sprintf("%s %s %s",
		caser.String(bvnDetails.Data.Details.FirstName),
		caser.String(bvnDetails.Data.Details.MiddleName),
		caser.String(bvnDetails.Data.Details.LastName))

	verificationID := uuid.NewString()
	subjectHashBytes := sha256.Sum256([]byte(strings.TrimSpace(bvn)))
	subjectHash := hex.EncodeToString(subjectHashBytes[:])
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	maskedBVN := MaskSub(bvn)

	record := &models.VerificationRecord{
		ID:            verificationID,
		Type:          models.VerificationTypeBVN,
		Provider:      string(ProviderTendar),
		Status:        models.VerificationStatusVerified,
		SubjectHash:   subjectHash,
		SubjectMasked: &maskedBVN,
		VerifiedAt:    &now,
		ExpiresAt:     &expiresAt,
		VerifiedName:  &fullName,
		VerifiedDOB:   &bvnDetails.Data.Details.DateOfBirth,
		VerifiedID:    &bvn,
	}

	if providerVerificationID := strings.TrimSpace(bvnDetails.VerificationID); providerVerificationID != "" {
		log.Printf("tendar verification id: %s\n", providerVerificationID)
		record.ProviderVerificationID = &providerVerificationID
	}
	if fullName != "" {
		record.VerifiedName = &fullName
	}
	if phone := strings.TrimSpace(bvnDetails.Data.Details.PhoneNumber); phone != "" {
		normalizedPhoneNumber, err := NormalizeNigerianNumber(phone)
		if err != nil {
			return nil, err
		}
		record.VerifiedPhone = &normalizedPhoneNumber
	}
	if email := strings.TrimSpace(bvnDetails.Data.Details.Email); email != "" {
		record.VerifiedEmail = &email
	}
	if dob := strings.TrimSpace(bvnDetails.Data.Details.DateOfBirth); dob != "" {
		record.VerifiedDOB = &dob
	}

	if fullName == "" || bvnDetails.Data.Details.DateOfBirth == "" || bvnDetails.Data.Details.PhoneNumber == "" {
		log.Printf("invalid bvn number")
		return nil, errors.New("invalid bvn number")
	}

	bvnRecord := &models.BVNRecord{
		ID:              uuid.NewString(),
		UserID:          "",
		FirstName:       strings.TrimSpace(caser.String(bvnDetails.Data.Details.FirstName)),
		MiddleName:      strings.TrimSpace(caser.String(bvnDetails.Data.Details.MiddleName)),
		LastName:        strings.TrimSpace(caser.String(bvnDetails.Data.Details.LastName)),
		Gender:          strings.TrimSpace(bvnDetails.Data.Details.Gender),
		Nationality:     strings.TrimSpace(bvnDetails.Data.Details.Nationality),
		StateOfOrigin:   strings.TrimSpace(bvnDetails.Data.Details.StateOfOrigin),
		DateOfBirth:     parseBVNRecordDOB(bvnDetails.Data.Details.DateOfBirth),
		PlaceOfBirth:    "",
		Occupation:      "",
		MaritalStatus:   strings.TrimSpace(bvnDetails.Data.Details.MaritalStatus),
		Education:       "",
		Religion:        "",
		EmailAddress:    firstNonEmptyString(bvnDetails.Data.Details.Email, bvnDetails.Data.Email),
		PassportOnBVN:   strings.TrimSpace(bvnDetails.Data.Details.Image),
		FullHomeAddress: strings.TrimSpace(bvnDetails.Data.Details.ResidentialAddress),
		MobilePhone:     firstNonEmptyString(bvnDetails.Data.Details.PhoneNumber, bvnDetails.Data.PhoneNumber),
		BankName:        strings.TrimSpace(bvnDetails.Data.Details.EnrollmentBank),
		BVN:             strings.TrimSpace(bvn),
	}
	bvnRecord.AlternativeMobilePhone = trimmedStringPtr(bvnDetails.Data.Details.PhoneNumber2)
	bvnRecord.City = trimmedStringPtr(bvnDetails.Data.Details.LGAOfResidence)

	if err := s.saveVerifiedBVN(ctx, record, bvnRecord); err != nil {
		log.Printf("failed to add verification record err=%v", err)
		return nil, err
	}

	return &bvnInfo{
		name:           fullName,
		dob:            bvnDetails.Data.Details.DateOfBirth,
		phone:          bvnDetails.Data.Details.PhoneNumber,
		verificationID: verificationID,
	}, nil
}

func (s *Service) ValidateBVNWithPrembly(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.prembly == nil {
		return nil, errors.New("couldn't resolve prembly provider")
	}

	if bvn == "" {
		return nil, errors.New("bvn is required")
	}

	if len(bvn) != 11 {
		return nil, errors.New("invalid bvn number")
	}

	bvnDetails, err := s.prembly.ValidateBVNWithPrembly(ctx, bvn)
	if err != nil {
		return nil, err
	}
	if bvnDetails == nil {
		return nil, errors.New("invalid bvn number")
	}

	firstName := TitleCase(bvnDetails.Data.FirstName)
	middleName := TitleCase(bvnDetails.Data.MiddleName)
	lastName := TitleCase(bvnDetails.Data.LastName)
	fullName := strings.Join(strings.Fields(fmt.Sprintf(
		"%s %s %s",
		firstName,
		middleName,
		lastName,
	)), " ")
	verificationID := uuid.NewString()
	subjectHashBytes := sha256.Sum256([]byte(strings.TrimSpace(bvn)))
	subjectHash := hex.EncodeToString(subjectHashBytes[:])
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	maskedBVN := MaskSub(bvn)

	record := &models.VerificationRecord{
		ID:            verificationID,
		Type:          models.VerificationTypeBVN,
		Provider:      string(ProviderPrembly),
		Status:        models.VerificationStatusVerified,
		SubjectHash:   subjectHash,
		SubjectMasked: &maskedBVN,
		VerifiedAt:    &now,
		ExpiresAt:     &expiresAt,
		VerifiedName:  &fullName,
		VerifiedDOB:   &bvnDetails.Data.DateOfBirth,
		VerifiedID:    &bvn,
	}

	if providerVerificationID := strings.TrimSpace(bvnDetails.Verification.VerificationID); providerVerificationID != "" {
		record.ProviderVerificationID = &providerVerificationID
	}
	if referenceID := strings.TrimSpace(bvnDetails.ReferenceID); referenceID != "" {
		record.ReferenceID = &referenceID
	}
	if fullName != "" {
		record.VerifiedName = &fullName
	}
	if phone := strings.TrimSpace(bvnDetails.Data.PhoneNumber); phone != "" {
		normalizedPhoneNumber, err := NormalizeNigerianNumber(phone)
		if err != nil {
			return nil, err
		}
		record.VerifiedPhone = &normalizedPhoneNumber
	}
	if email := strings.TrimSpace(bvnDetails.Data.Email); email != "" {
		record.VerifiedEmail = &email
	}
	if dob := strings.TrimSpace(bvnDetails.Data.DateOfBirth); dob != "" {
		record.VerifiedDOB = &dob
	}

	if fullName == "" || bvnDetails.Data.DateOfBirth == "" || bvnDetails.Data.PhoneNumber == "" {
		return nil, errors.New("invalid bvn number")
	}

	bvnRecord := &models.BVNRecord{
		ID:              uuid.NewString(),
		UserID:          "",
		FirstName:       firstName,
		MiddleName:      middleName,
		LastName:        lastName,
		Gender:          strings.TrimSpace(bvnDetails.Data.Gender),
		Nationality:     strings.TrimSpace(bvnDetails.Data.Nationality),
		StateOfOrigin:   strings.TrimSpace(bvnDetails.Data.StateOfOrigin),
		DateOfBirth:     parseBVNRecordDOB(bvnDetails.Data.DateOfBirth),
		PlaceOfBirth:    "",
		Occupation:      "",
		MaritalStatus:   strings.TrimSpace(bvnDetails.Data.MaritalStatus),
		Education:       "",
		Religion:        "",
		EmailAddress:    strings.TrimSpace(bvnDetails.Data.Email),
		PassportOnBVN:   trimmedStringValue(bvnDetails.Data.Image),
		FullHomeAddress: "",
		MobilePhone:     strings.TrimSpace(bvnDetails.Data.PhoneNumber),
		BankName:        strings.TrimSpace(bvnDetails.Data.EnrollmentBank),
		BVN:             strings.TrimSpace(firstNonEmptyString(bvnDetails.Data.BVN, bvn)),
	}

	if err := s.saveVerifiedBVN(ctx, record, bvnRecord); err != nil {
		return nil, err
	}

	return &bvnInfo{
		name:           fullName,
		dob:            bvnDetails.Data.DateOfBirth,
		phone:          bvnDetails.Data.PhoneNumber,
		verificationID: verificationID,
	}, nil
}

func (s *Service) saveVerifiedBVN(ctx context.Context, verificationRecord *models.VerificationRecord, bvnRecord *models.BVNRecord) error {
	if s.verification == nil {
		return errors.New("verification repository not configured")
	}
	if s.repo == nil {
		return errors.New("auth repository not configured")
	}
	if s.tx == nil {
		if err := s.verification.AddVerification(ctx, verificationRecord); err != nil {
			return err
		}
		return s.repo.CreateBVNRecord(ctx, bvnRecord)
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		authRepo := NewRespository(txDB)
		verificationRepo := verification.NewVerification(txDB)

		if err := verificationRepo.AddVerification(ctx, verificationRecord); err != nil {
			return err
		}

		return authRepo.CreateBVNRecord(ctx, bvnRecord)
	})
}
