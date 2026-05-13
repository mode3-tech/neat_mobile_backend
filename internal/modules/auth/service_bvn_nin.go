package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/modules/auth/verification"
	phoneUtil "neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/models"
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

	row, err := s.repo.GetValidationRow(ctx, bvnVerificationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrBVNNotFound
		}
		return nil, err
	}

	if info, err := s.reuseVerifiedNIN(ctx, nin, row); err == nil && info != nil {
		return info, nil
	}

	resp, err := s.nin.ValidateNIN(ctx, nin)
	if err != nil {
		log.Printf("ValidateNIN: provider call failed: %v", err)
		return nil, err
	}

	firstName := TitleCase(resp.Data.FirstName)
	middleName := TitleCase(resp.Data.MiddleName)
	lastName := TitleCase(resp.Data.Surname)
	fullName := fmt.Sprintf("%s %s %s", firstName, middleName, lastName)

	_, err = compareBVNAndNinDetails(*row.VerifiedName, SerializeDOB(strings.TrimSpace(*row.VerifiedDOB)), fullName, SerializeDOB(strings.TrimSpace(resp.Data.BirthDate)))

	if err != nil {
		log.Printf("ValidateNIN: BVN/NIN detail mismatch: %v", err)
		return nil, errors.New(err.Error())
	}

	verificationID := uuid.NewString()
	subjectHashBytes := sha256.Sum256([]byte(strings.TrimSpace(nin)))
	subjectHash := hex.EncodeToString(subjectHashBytes[:])
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	maskedNIN := MaskSub(nin)
	normalizedPhoneNumber, err := phoneUtil.NormalizeNigerianNumber(resp.Data.TelephoneNo)
	if err != nil {
		log.Printf("ValidateNIN: phone normalization failed phone=%q err=%v", resp.Data.TelephoneNo, err)
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
		return nil, appErr.ErrInvalidNIN
	}

	record.VerifiedName = &fullName
	dob := strings.TrimSpace(resp.Data.BirthDate)
	record.VerifiedDOB = &dob
	phone := strings.TrimSpace(resp.Data.TelephoneNo)
	record.VerifiedPhone = &phone

	if err := s.verification.AddVerification(ctx, record); err != nil {
		log.Printf("ValidateNIN: AddVerification failed: %v", err)
		return nil, err
	}

	maskedPhone, err := phoneUtil.MaskPhone(phone)
	if err != nil {
		log.Printf("ValidateNIN: MaskPhone failed: %v", err)
		return nil, err
	}

	return &ninInfo{
		name:           fullName,
		dob:            dob,
		phone:          maskedPhone,
		verificationID: verificationID,
	}, nil
}

func (s *Service) ValidateBVN(ctx context.Context, bvn string) (*bvnInfo, error) {
	if info, err := s.reuseVerifiedBVN(ctx, bvn); err == nil && info != nil {
		return info, nil
	}

	if s.providerSource == nil {
		return s.ValidateBVNWithTendar(ctx, bvn)
	}

	provider, err := s.providerSource.GetCurrentProvider(ctx)
	if err != nil {
		log.Printf("failed to resolve bvn provider from source; forcing tendar: %v", err)
		return s.ValidateBVNWithTendar(ctx, bvn)
	}

	switch provider {
	case ProviderPrembly:
		return s.ValidateBVNWithPrembly(ctx, bvn)
	default:
		return s.ValidateBVNWithTendar(ctx, bvn)
	}
}

func (s *Service) reuseVerifiedBVN(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.verification == nil {
		return nil, errors.New("verification repo not configured")
	}
	hashBytes := sha256.Sum256([]byte(strings.TrimSpace(bvn)))
	hash := hex.EncodeToString(hashBytes[:])

	cached, err := s.verification.GetVerifiedRecordBySubjectHash(ctx, hash, models.VerificationTypeBVN)
	if err != nil || cached == nil {
		return nil, errors.New("no cached bvn record")
	}
	if cached.VerifiedName == nil || cached.VerifiedDOB == nil || cached.VerifiedPhone == nil {
		return nil, errors.New("cached bvn record is incomplete")
	}

	verificationID := uuid.NewString()
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	maskedBVN := MaskSub(bvn)

	record := &models.VerificationRecord{
		ID:                      verificationID,
		Type:                    models.VerificationTypeBVN,
		Provider:                cached.Provider,
		Status:                  models.VerificationStatusVerified,
		SubjectHash:             hash,
		SubjectMasked:           &maskedBVN,
		VerifiedAt:              &now,
		ExpiresAt:               &expiresAt,
		VerifiedID:              cached.VerifiedID,
		VerifiedName:            cached.VerifiedName,
		VerifiedDOB:             cached.VerifiedDOB,
		VerifiedPhone:           cached.VerifiedPhone,
		VerifiedEmail:           cached.VerifiedEmail,
		VerifiedGender:          cached.VerifiedGender,
		VerifiedNationality:     cached.VerifiedNationality,
		VerifiedStateOfOrigin:   cached.VerifiedStateOfOrigin,
		VerifiedPlaceOfBirth:    cached.VerifiedPlaceOfBirth,
		VerifiedOccupation:      cached.VerifiedOccupation,
		VerifiedMaritalStatus:   cached.VerifiedMaritalStatus,
		VerifiedEducation:       cached.VerifiedEducation,
		VerifiedReligion:        cached.VerifiedReligion,
		PassportOnBVN:           cached.PassportOnBVN,
		Passport:                cached.Passport,
		VerifiedFullHomeAddress: cached.VerifiedFullHomeAddress,
		TypeOfHouse:             cached.TypeOfHouse,
		City:                    cached.City,
		Landmark:                cached.Landmark,
		LivingSince:             cached.LivingSince,
		AlternativeMobilePhone:  cached.AlternativeMobilePhone,
		BankName:                cached.BankName,
		AccountNumber:           cached.AccountNumber,
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	firstName, middleName, lastName := SplitFullName(*cached.VerifiedName)
	bvnRecord := &models.BVNRecord{
		ID:                     uuid.NewString(),
		UserID:                 "",
		FirstName:              firstName,
		MiddleName:             middleName,
		LastName:               lastName,
		Gender:                 derefString(cached.VerifiedGender),
		Nationality:            derefString(cached.VerifiedNationality),
		StateOfOrigin:          derefString(cached.VerifiedStateOfOrigin),
		PlaceOfBirth:           derefString(cached.VerifiedPlaceOfBirth),
		Occupation:             derefString(cached.VerifiedOccupation),
		MaritalStatus:          derefString(cached.VerifiedMaritalStatus),
		Education:              derefString(cached.VerifiedEducation),
		Religion:               derefString(cached.VerifiedReligion),
		EmailAddress:           derefString(cached.VerifiedEmail),
		PassportOnBVN:          derefString(cached.PassportOnBVN),
		Passport:               cached.Passport,
		FullHomeAddress:        derefString(cached.VerifiedFullHomeAddress),
		TypeOfHouse:            cached.TypeOfHouse,
		City:                   cached.City,
		Landmark:               cached.Landmark,
		LivingSince:            cached.LivingSince,
		MobilePhone:            *cached.VerifiedPhone,
		AlternativeMobilePhone: cached.AlternativeMobilePhone,
		BankName:               cached.BankName,
		AccountNumber:          cached.AccountNumber,
		DateOfBirth:            parseBVNRecordDOB(*cached.VerifiedDOB),
		BVN:                    *cached.VerifiedID,
	}

	if err := s.saveVerifiedBVN(ctx, record, bvnRecord); err != nil {
		log.Printf("reuseVerifiedBVN: saveVerifiedBVN failed: %v", err)
		return nil, err
	}

	log.Printf("bvn cache hit: reused existing verified record masked=%s", maskedBVN)

	maskedPhone, err := phoneUtil.MaskPhone(*cached.VerifiedPhone)
	if err != nil {
		log.Printf("reuseVerifiedBVN: MaskPhone failed: %v", err)
		return nil, err
	}

	return &bvnInfo{
		name:           *cached.VerifiedName,
		dob:            *cached.VerifiedDOB,
		phone:          maskedPhone,
		verificationID: verificationID,
	}, nil
}

func (s *Service) reuseVerifiedNIN(ctx context.Context, nin string, bvnRow *models.VerificationRecord) (*ninInfo, error) {
	if s.verification == nil {
		return nil, errors.New("verification repo not configured")
	}
	hashBytes := sha256.Sum256([]byte(strings.TrimSpace(nin)))
	hash := hex.EncodeToString(hashBytes[:])

	cached, err := s.verification.GetVerifiedRecordBySubjectHash(ctx, hash, models.VerificationTypeNIN)
	if err != nil || cached == nil {
		return nil, errors.New("no cached nin record")
	}
	if cached.VerifiedName == nil || cached.VerifiedDOB == nil || cached.VerifiedPhone == nil {
		return nil, errors.New("cached nin record is incomplete")
	}

	if _, err := compareBVNAndNinDetails(
		*bvnRow.VerifiedName,
		SerializeDOB(strings.TrimSpace(*bvnRow.VerifiedDOB)),
		*cached.VerifiedName,
		SerializeDOB(strings.TrimSpace(*cached.VerifiedDOB)),
	); err != nil {
		return nil, errors.New(err.Error())
	}

	verificationID := uuid.NewString()
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	maskedNIN := MaskSub(nin)

	record := &models.VerificationRecord{
		ID:            verificationID,
		Type:          models.VerificationTypeNIN,
		Provider:      cached.Provider,
		Status:        models.VerificationStatusVerified,
		SubjectHash:   hash,
		SubjectMasked: &maskedNIN,
		VerifiedAt:    &now,
		ExpiresAt:     &expiresAt,
		VerifiedID:    cached.VerifiedID,
		VerifiedName:  cached.VerifiedName,
		VerifiedDOB:   cached.VerifiedDOB,
		VerifiedPhone: cached.VerifiedPhone,
		VerifiedEmail: cached.VerifiedEmail,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.verification.AddVerification(ctx, record); err != nil {
		log.Printf("reuseVerifiedNIN: AddVerification failed: %v", err)
		return nil, err
	}

	log.Printf("nin cache hit: reused existing verified record masked=%s", maskedNIN)

	maskedPhone, err := phoneUtil.MaskPhone(*cached.VerifiedPhone)
	if err != nil {
		log.Printf("reuseVerifiedNIN: MaskPhone failed: %v", err)
		return nil, err
	}

	return &ninInfo{
		name:           *cached.VerifiedName,
		dob:            *cached.VerifiedDOB,
		phone:          maskedPhone,
		verificationID: verificationID,
	}, nil
}

func (s *Service) ValidateBVNWithTendar(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.tender == nil {
		log.Printf("tendar validator is not configured")
		return nil, errors.New("tendar validator is not configured")
	}

	if bvn == "" {
		log.Printf("bvn is required")
		return nil, appErr.ErrInvalidBVN
	}

	if len(bvn) != 11 {
		log.Printf("invalid bvn number")
		return nil, appErr.ErrInvalidBVN
	}

	bvnDetails, err := s.tender.ValidateBVNWithTendar(ctx, bvn)
	if err != nil {
		log.Printf("ValidateBVNWithTendar: provider call failed: %v", err)
		return nil, err
	}
	if bvnDetails == nil {
		log.Printf("invalid bvn number")
		return nil, appErr.ErrInvalidBVN
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
		normalizedPhoneNumber, err := phoneUtil.NormalizeNigerianNumber(phone)
		if err != nil {
			log.Printf("ValidateBVNWithTendar: phone normalization failed phone=%q err=%v", phone, err)
			return nil, appErr.ErrInvalidBVN
		}
		record.VerifiedPhone = &normalizedPhoneNumber
	}
	if email := strings.TrimSpace(bvnDetails.Data.Details.Email); email != "" {
		record.VerifiedEmail = &email
	}
	if dob := strings.TrimSpace(bvnDetails.Data.Details.DateOfBirth); dob != "" {
		record.VerifiedDOB = &dob
	}
	if gender := strings.TrimSpace(bvnDetails.Data.Details.Gender); gender != "" {
		record.VerifiedGender = &gender
	}
	if v := strings.TrimSpace(bvnDetails.Data.Details.Nationality); v != "" {
		record.VerifiedNationality = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.Details.StateOfOrigin); v != "" {
		record.VerifiedStateOfOrigin = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.Details.MaritalStatus); v != "" {
		record.VerifiedMaritalStatus = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.Details.Image); v != "" {
		record.PassportOnBVN = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.Details.ResidentialAddress); v != "" {
		record.VerifiedFullHomeAddress = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.Details.LGAOfResidence); v != "" {
		record.City = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.Details.EnrollmentBank); v != "" {
		record.BankName = v
	}
	if v := strings.TrimSpace(bvnDetails.Data.Details.PhoneNumber2); v != "" {
		record.AlternativeMobilePhone = &v
	}

	if fullName == "" || bvnDetails.Data.Details.DateOfBirth == "" || bvnDetails.Data.Details.PhoneNumber == "" {
		log.Printf("invalid bvn number")
		return nil, appErr.ErrInvalidBVN
	}

	if record.VerifiedPhone == nil {
		log.Printf("ValidateBVNWithTendar: verified phone is nil bvn=%s", MaskSub(bvn))
		return nil, appErr.ErrInvalidBVN
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
		MobilePhone:     *record.VerifiedPhone,
		BankName:        strings.TrimSpace(bvnDetails.Data.Details.EnrollmentBank),
		BVN:             strings.TrimSpace(bvn),
	}
	bvnRecord.AlternativeMobilePhone = trimmedStringPtr(bvnDetails.Data.Details.PhoneNumber2)
	bvnRecord.City = trimmedStringPtr(bvnDetails.Data.Details.LGAOfResidence)

	if err := s.saveVerifiedBVN(ctx, record, bvnRecord); err != nil {
		log.Printf("failed to add verification record err=%v", err)
		return nil, err
	}

	maskedPhone, err := phoneUtil.MaskPhone(*record.VerifiedPhone)
	if err != nil {
		log.Printf("ValidateBVNWithTendar: MaskPhone failed: %v", err)
		return nil, err
	}

	return &bvnInfo{
		name:           fullName,
		dob:            bvnDetails.Data.Details.DateOfBirth,
		phone:          maskedPhone,
		verificationID: verificationID,
	}, nil
}

func (s *Service) ValidateBVNWithPrembly(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.prembly == nil {
		log.Printf("ValidateBVNWithPrembly: prembly provider not configured")
		return nil, errors.New("couldn't resolve prembly provider")
	}

	if bvn == "" {
		return nil, appErr.ErrInvalidBVN
	}

	if len(bvn) != 11 {
		return nil, appErr.ErrInvalidBVN
	}

	bvnDetails, err := s.prembly.ValidateBVNWithPrembly(ctx, bvn)
	if err != nil {
		log.Printf("ValidateBVNWithPrembly: provider call failed: %v", err)
		return nil, err
	}
	if bvnDetails == nil {
		log.Printf("ValidateBVNWithPrembly: provider returned nil response")
		return nil, appErr.ErrInvalidBVN
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
	if phone := strings.TrimSpace(bvnDetails.Data.PhoneNumber1); phone != "" {
		normalizedPhoneNumber, err := phoneUtil.NormalizeNigerianNumber(phone)
		if err != nil {
			log.Printf("ValidateBVNWithPrembly: phone normalization failed phone=%q err=%v", phone, err)
			return nil, appErr.ErrInvalidBVN
		}
		record.VerifiedPhone = &normalizedPhoneNumber
	}
	if email := strings.TrimSpace(bvnDetails.Data.Email); email != "" {
		record.VerifiedEmail = &email
	}
	if dob := strings.TrimSpace(bvnDetails.Data.DateOfBirth); dob != "" {
		record.VerifiedDOB = &dob
	}
	if gender := strings.TrimSpace(bvnDetails.Data.Gender); gender != "" {
		record.VerifiedGender = &gender
	}
	if v := strings.TrimSpace(bvnDetails.Data.Nationality); v != "" {
		record.VerifiedNationality = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.StateOfOrigin); v != "" {
		record.VerifiedStateOfOrigin = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.MaritalStatus); v != "" {
		record.VerifiedMaritalStatus = &v
	}
	if v := trimmedStringValue(bvnDetails.Data.Image); v != "" {
		record.PassportOnBVN = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.LGAOfOrigin); v != "" {
		record.City = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.ResidentialAddress); v != "" {
		record.VerifiedFullHomeAddress = &v
	}
	if v := strings.TrimSpace(bvnDetails.Data.EnrollmentBank); v != "" {
		record.BankName = v
	}

	if fullName == "" || bvnDetails.Data.DateOfBirth == "" || bvnDetails.Data.PhoneNumber1 == "" {
		log.Printf("ValidateBVNWithPrembly: incomplete response fullName=%q dob=%q phone=%q", fullName, bvnDetails.Data.DateOfBirth, bvnDetails.Data.PhoneNumber1)
		return nil, appErr.ErrInvalidBVN
	}

	if record.VerifiedPhone == nil {
		log.Printf("ValidateBVNWithPrembly: verified phone is nil bvn=%s", MaskSub(bvn))
		return nil, appErr.ErrInvalidBVN
	}

	bvnRecord := &models.BVNRecord{
		ID:            uuid.NewString(),
		UserID:        "",
		FirstName:     firstName,
		MiddleName:    middleName,
		LastName:      lastName,
		Gender:        strings.TrimSpace(bvnDetails.Data.Gender),
		Nationality:   strings.TrimSpace(bvnDetails.Data.Nationality),
		StateOfOrigin: strings.TrimSpace(bvnDetails.Data.StateOfOrigin),
		DateOfBirth:   parseBVNRecordDOB(bvnDetails.Data.DateOfBirth),
		MaritalStatus: strings.TrimSpace(bvnDetails.Data.MaritalStatus),
		EmailAddress:  strings.TrimSpace(bvnDetails.Data.Email),
		PassportOnBVN: trimmedStringValue(bvnDetails.Data.Image),
		MobilePhone:   *record.VerifiedPhone,
		BankName:      strings.TrimSpace(bvnDetails.Data.EnrollmentBank),
		BVN:           strings.TrimSpace(firstNonEmptyString(bvnDetails.Data.BVN, bvn)),
	}
	bvnRecord.City = trimmedStringPtr(bvnDetails.Data.LGAOfOrigin)

	if err := s.saveVerifiedBVN(ctx, record, bvnRecord); err != nil {
		log.Printf("ValidateBVNWithPrembly: saveVerifiedBVN failed: %v", err)
		return nil, err
	}

	maskedPhone, err := phoneUtil.MaskPhone(*record.VerifiedPhone)
	if err != nil {
		log.Printf("ValidateBVNWithPrembly: MaskPhone failed: %v", err)
		return nil, err
	}

	return &bvnInfo{
		name:           fullName,
		dob:            bvnDetails.Data.DateOfBirth,
		phone:          maskedPhone,
		verificationID: verificationID,
	}, nil
}

func (s *Service) saveVerifiedBVN(ctx context.Context, verificationRecord *models.VerificationRecord, bvnRecord *models.BVNRecord) error {
	if s.verification == nil {
		log.Printf("saveVerifiedBVN: verification repo is nil")
		return errors.New("verification repository not configured")
	}
	if s.repo == nil {
		log.Printf("saveVerifiedBVN: auth repo is nil")
		return errors.New("auth repository not configured")
	}
	if s.tx == nil {
		if err := s.verification.AddVerification(ctx, verificationRecord); err != nil {
			log.Printf("saveVerifiedBVN: AddVerification failed: %v", err)
			return err
		}
		if bvnRecord != nil {
			if err := s.repo.CreateBVNRecord(ctx, bvnRecord); err != nil {
				log.Printf("saveVerifiedBVN: CreateBVNRecord failed: %v", err)
				return err
			}
		}
		return nil
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		authRepo := NewRespository(txDB)
		verificationRepo := verification.NewVerification(txDB)

		if err := verificationRepo.AddVerification(ctx, verificationRecord); err != nil {
			log.Printf("saveVerifiedBVN: tx AddVerification failed: %v", err)
			return err
		}

		if bvnRecord != nil {
			if err := authRepo.CreateBVNRecord(ctx, bvnRecord); err != nil {
				log.Printf("saveVerifiedBVN: tx CreateBVNRecord failed: %v", err)
				return err
			}
		}
		return nil
	})
}
