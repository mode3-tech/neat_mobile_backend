package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/email"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/helpers"
	"neat_mobile_app_backend/internal/middleware"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"
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

// logVerificationAudit records a BVN/NIN verification attempt. These flows run before
// a user account exists, so there's no real user ID to attribute the action to —
// resourceID doubles as ActorID, using whatever identity is available at that point
// in the flow (the inbound verification ID, or a masked subject once one is minted).
func (s *Service) logVerificationAudit(ctx context.Context, resourceType auditlog.ResourceType, action, resourceID string, status auditlog.LogStatus, metadata map[string]interface{}) {
	s.auditLogger.CreateAuditLog(ctx, &auditlog.AuditLog{
		ID:           helpers.PrefixID("audit_log"),
		ResourceID:   resourceID,
		ResourceType: resourceType,
		ActorType:    "unauthenticated",
		RequestID:    middleware.GetRequestID(ctx),
		Timestamp:    time.Now().UTC(),
		Status:       status,
		ActorID:      resourceID,
		Action:       action,
		IPAddress:    middleware.GetClientIP(ctx),
		Metadata:     metadata,
	})
}

// ValidateNIN resolves the configured identity provider and performs the NIN lookup.
// The same system_preferences row drives BVN and NIN, so flipping it moves both.
func (s *Service) ValidateNIN(ctx context.Context, bvnVerificationID, nin string) (*ninInfo, error) {
	provider, client := s.ninProviderFor(ctx)
	if client == nil {
		log.Printf("ValidateNIN: %s nin provider is not configured", provider)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", bvnVerificationID, auditlog.StatusFailure, map[string]interface{}{
			"NIN":                MaskSub(nin),
			"provider":           provider,
			"reason for failure": "provider is not configured",
		})
		return nil, appErr.ErrProviderServiceUnavailable
	}

	return s.validateNINWith(ctx, provider, client, bvnVerificationID, nin)
}

// ninProviderFor resolves the configured provider and the client that serves it.
// Anything other than an explicit prembly preference lands on Tendar.
func (s *Service) ninProviderFor(ctx context.Context) (Provider, NINValidation) {
	if s.resolveProvider(ctx) == ProviderPrembly {
		return ProviderPrembly, s.ninPrembly
	}
	return ProviderTendar, s.ninTendar
}

func (s *Service) validateNINWith(ctx context.Context, provider Provider, client NINValidation, bvnVerificationID, nin string) (*ninInfo, error) {
	if len(nin) != 11 {
		s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", bvnVerificationID, auditlog.StatusFailure, map[string]interface{}{
			"NIN":                MaskSub(nin),
			"reason for failure": "invalid NIN",
		})
		return nil, appErr.ErrInvalidNIN
	}

	row, err := s.repo.GetValidationRow(ctx, bvnVerificationID)
	if err != nil {
		reason := "failed to load bvn verification row"
		if errors.Is(err, gorm.ErrRecordNotFound) {
			reason = "bvn verification not found"
		}
		s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", bvnVerificationID, auditlog.StatusFailure, map[string]interface{}{
			"NIN":                MaskSub(nin),
			"reason for failure": reason,
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrBVNNotFound
		}
		return nil, err
	}

	resp, err := client.ValidateNIN(ctx, nin)
	if err != nil {
		log.Printf("ValidateNIN: %s provider call failed: %v", provider, err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", bvnVerificationID, auditlog.StatusFailure, map[string]interface{}{
			"NIN":                MaskSub(nin),
			"provider":           provider,
			"reason for failure": "provider call failed",
		})
		return nil, translateProviderError(err, appErr.ErrNINNotFound)
	}

	firstName := TitleCase(resp.Data.FirstName)
	middleName := TitleCase(resp.Data.MiddleName)
	lastName := TitleCase(resp.Data.Surname)
	fullName := fmt.Sprintf("%s %s %s", firstName, middleName, lastName)

	_, err = compareBVNAndNinDetails(*row.VerifiedName, strings.TrimSpace(*row.VerifiedDOB), fullName, strings.TrimSpace(resp.Data.BirthDate))

	if err != nil {
		log.Printf("ValidateNIN: BVN/NIN detail mismatch: %v", err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", bvnVerificationID, auditlog.StatusFailure, map[string]interface{}{
			"NIN":                MaskSub(nin),
			"reason for failure": "bvn/nin detail mismatch",
		})
		return nil, appErr.ErrNINAndBVNMismatch
	}

	verificationID := uuid.NewString()
	subjectHashBytes := sha256.Sum256([]byte(strings.TrimSpace(nin)))
	subjectHash := hex.EncodeToString(subjectHashBytes[:])
	now := time.Now().UTC()
	expiresAt := now.Add(30 * time.Minute)
	maskedNIN := MaskSub(nin)
	normalizedPhoneNumber, err := phoneUtil.NormalizeNigerianNumber(resp.Data.TelephoneNo)
	if err != nil {
		log.Printf("ValidateNIN: phone normalization failed phone=%q err=%v", resp.Data.TelephoneNo, err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"NIN":                maskedNIN,
			"reason for failure": "phone normalization failed",
		})
		return nil, err
	}

	record := &models.VerificationRecord{
		ID:            verificationID,
		Type:          models.VerificationTypeNIN,
		Status:        models.VerificationStatusVerified,
		Provider:      string(provider),
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
		s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"NIN":                maskedNIN,
			"reason for failure": "incomplete provider response",
		})
		return nil, appErr.ErrInvalidNIN
	}

	record.VerifiedName = &fullName
	dob := strings.TrimSpace(resp.Data.BirthDate)
	record.VerifiedDOB = &dob
	phone := strings.TrimSpace(resp.Data.TelephoneNo)
	record.VerifiedPhone = &phone

	if err := s.verification.AddVerification(ctx, record); err != nil {
		log.Printf("ValidateNIN: AddVerification failed: %v", err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"NIN":                maskedNIN,
			"reason for failure": "failed to persist verification record",
		})
		return nil, err
	}

	maskedPhone, err := phoneUtil.MaskPhone(phone)
	if err != nil {
		log.Printf("ValidateNIN: MaskPhone failed: %v", err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"NIN":                maskedNIN,
			"reason for failure": "failed to mask phone",
		})
		return nil, err
	}

	s.logVerificationAudit(ctx, auditlog.ResourceTypeNIN, "NIN_VALIDATION", verificationID, auditlog.StatusSuccess, map[string]interface{}{
		"NIN":      maskedNIN,
		"provider": provider,
	})

	return &ninInfo{
		name:           fullName,
		dob:            dob,
		phone:          maskedPhone,
		verificationID: verificationID,
	}, nil
}

// resolveProvider reads the single system_preferences row that governs both BVN and
// NIN validation. Any failure — no source wired, missing row, DB error — falls back
// to Tendar.
func (s *Service) resolveProvider(ctx context.Context) Provider {
	if s.providerSource == nil {
		return ProviderTendar
	}

	provider, err := s.providerSource.GetCurrentProvider(ctx)
	if err != nil {
		log.Printf("failed to resolve validation provider from source; forcing tendar: %v", err)
		return ProviderTendar
	}
	return provider
}

func (s *Service) ValidateBVN(ctx context.Context, bvn string) (*bvnInfo, error) {
	switch s.resolveProvider(ctx) {
	case ProviderPrembly:
		return s.ValidateBVNWithPrembly(ctx, bvn)
	default:
		return s.ValidateBVNWithTendar(ctx, bvn)
	}
}

func (s *Service) ValidateBVNWithTendar(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.tender == nil {
		log.Printf("tendar validator is not configured")
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"BVN":                MaskSub(bvn),
			"provider":           ProviderTendar,
			"reason for failure": "provider is not configured",
		})
		return nil, appErr.ErrProviderServiceUnavailable
	}

	if bvn == "" {
		log.Printf("bvn is required")
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"reason for failure": "bvn is required",
		})
		return nil, appErr.ErrInvalidBVN
	}

	if len(bvn) != 11 {
		log.Printf("invalid bvn number")
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"BVN":                MaskSub(bvn),
			"reason for failure": "invalid bvn",
		})
		return nil, appErr.ErrInvalidBVN
	}

	bvnDetails, err := s.tender.ValidateBVNWithTendar(ctx, bvn)
	if err != nil {
		log.Printf("ValidateBVNWithTendar: provider call failed: %v", err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"BVN":                MaskSub(bvn),
			"provider":           ProviderTendar,
			"reason for failure": "provider call failed",
		})
		return nil, translateProviderError(err, appErr.ErrBVNNotFound)
	}
	if bvnDetails == nil {
		log.Printf("invalid bvn number")
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"BVN":                MaskSub(bvn),
			"provider":           ProviderTendar,
			"reason for failure": "provider returned no data",
		})
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
	expiresAt := now.Add(30 * time.Minute)
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

	phone := strings.TrimSpace(bvnDetails.Data.Details.PhoneNumber)
	if phone == "" {
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderTendar,
			"reason for failure": "missing phone number",
		})
		return nil, appErr.ErrMissingBVNPhoneNumber
	}

	normalizedPhoneNumber, err := phoneUtil.NormalizeNigerianNumber(phone)
	if err != nil {
		log.Printf("ValidateBVNWithTendar: phone normalization failed phone=%q err=%v", phone, err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderTendar,
			"reason for failure": "phone normalization failed",
		})
		return nil, err
	}
	record.VerifiedPhone = &normalizedPhoneNumber
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
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderTendar,
			"reason for failure": "incomplete provider response",
		})
		return nil, appErr.ErrInvalidBVN
	}

	if record.VerifiedPhone == nil {
		log.Printf("ValidateBVNWithTendar: verified phone is nil bvn=%s", MaskSub(bvn))
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderTendar,
			"reason for failure": "verified phone is nil",
		})
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
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderTendar,
			"reason for failure": "failed to persist verification record",
		})
		return nil, err
	}

	maskedPhone, err := phoneUtil.MaskPhone(*record.VerifiedPhone)
	if err != nil {
		log.Printf("ValidateBVNWithTendar: MaskPhone failed: %v", err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderTendar,
			"reason for failure": "failed to mask phone",
		})
		return nil, err
	}

	maskedEmail := email.MaskEmail(bvnDetails.Data.Details.Email)

	s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusSuccess, map[string]interface{}{
		"BVN":      maskedBVN,
		"provider": ProviderTendar,
	})

	return &bvnInfo{
		name:           fullName,
		dob:            bvnDetails.Data.Details.DateOfBirth,
		phone:          maskedPhone,
		email:          maskedEmail,
		verificationID: verificationID,
	}, nil
}

func (s *Service) ValidateBVNWithPrembly(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.prembly == nil {
		log.Printf("ValidateBVNWithPrembly: prembly provider not configured")
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"BVN":                MaskSub(bvn),
			"provider":           ProviderPrembly,
			"reason for failure": "provider is not configured",
		})
		return nil, appErr.ErrProviderServiceUnavailable
	}

	if bvn == "" {
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"reason for failure": "bvn is required",
		})
		return nil, appErr.ErrInvalidBVN
	}

	if len(bvn) != 11 {
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"BVN":                MaskSub(bvn),
			"reason for failure": "invalid bvn",
		})
		return nil, appErr.ErrInvalidBVN
	}

	bvnDetails, err := s.prembly.ValidateBVNWithPrembly(ctx, bvn)
	if err != nil {
		log.Printf("ValidateBVNWithPrembly: provider call failed: %v", err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"BVN":                MaskSub(bvn),
			"provider":           ProviderPrembly,
			"reason for failure": "provider call failed",
		})
		return nil, translateProviderError(err, appErr.ErrBVNNotFound)
	}
	if bvnDetails == nil {
		log.Printf("ValidateBVNWithPrembly: provider returned nil response")
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", MaskSub(bvn), auditlog.StatusFailure, map[string]interface{}{
			"BVN":                MaskSub(bvn),
			"provider":           ProviderPrembly,
			"reason for failure": "provider returned no data",
		})
		return nil, appErr.ErrInvalidBVN
	}

	// Prembly non-deterministically serves BVN data from either data or bvn_data.
	// Merge both, preferring data and falling back to bvn_data for each field.
	d := bvnDetails.Data
	fb := bvnDetails.BVNData

	firstName := TitleCase(firstNonEmptyString(d.FirstName, fb.FirstName))
	middleName := TitleCase(firstNonEmptyString(d.MiddleName, fb.MiddleName))
	lastName := TitleCase(firstNonEmptyString(d.LastName, fb.LastName))
	fullName := strings.Join(strings.Fields(fmt.Sprintf(
		"%s %s %s",
		firstName,
		middleName,
		lastName,
	)), " ")
	dobField := firstNonEmptyString(d.DateOfBirth, fb.DateOfBirth)
	phoneField := firstNonEmptyString(d.PhoneNumber1, fb.PhoneNumber1)

	verificationID := uuid.NewString()
	subjectHashBytes := sha256.Sum256([]byte(strings.TrimSpace(bvn)))
	subjectHash := hex.EncodeToString(subjectHashBytes[:])
	now := time.Now().UTC()
	expiresAt := now.Add(30 * time.Minute)
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
		VerifiedDOB:   &dobField,
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
	if phone := strings.TrimSpace(phoneField); phone != "" {
		normalizedPhoneNumber, err := phoneUtil.NormalizeNigerianNumber(phone)
		if err != nil {
			log.Printf("ValidateBVNWithPrembly: phone normalization failed phone=%q err=%v", phone, err)
			s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
				"BVN":                maskedBVN,
				"provider":           ProviderPrembly,
				"reason for failure": "phone normalization failed",
			})
			return nil, appErr.ErrInvalidBVN
		}
		record.VerifiedPhone = &normalizedPhoneNumber
	}
	if email := firstNonEmptyString(d.Email, fb.Email); email != "" {
		record.VerifiedEmail = &email
	}
	if dob := strings.TrimSpace(dobField); dob != "" {
		record.VerifiedDOB = &dob
	}
	if gender := firstNonEmptyString(d.Gender, fb.Gender); gender != "" {
		record.VerifiedGender = &gender
	}
	if v := firstNonEmptyString(d.Nationality, fb.Nationality); v != "" {
		record.VerifiedNationality = &v
	}
	if v := firstNonEmptyString(d.StateOfOrigin, fb.StateOfOrigin); v != "" {
		record.VerifiedStateOfOrigin = &v
	}
	if v := firstNonEmptyString(d.MaritalStatus, fb.MaritalStatus); v != "" {
		record.VerifiedMaritalStatus = &v
	}
	if v := firstNonEmptyString(trimmedStringValue(d.Image), trimmedStringValue(fb.Image)); v != "" {
		record.PassportOnBVN = &v
	}
	if v := firstNonEmptyString(d.LGAOfOrigin, fb.LGAOfOrigin); v != "" {
		record.City = &v
	}
	if v := firstNonEmptyString(d.ResidentialAddress, fb.ResidentialAddress); v != "" {
		record.VerifiedFullHomeAddress = &v
	}
	if v := firstNonEmptyString(d.EnrollmentBank, fb.EnrollmentBank); v != "" {
		record.BankName = v
	}

	if fullName == "" || dobField == "" || phoneField == "" {
		log.Printf("ValidateBVNWithPrembly: incomplete response fullName=%q dob=%q phone=%q", fullName, dobField, phoneField)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderPrembly,
			"reason for failure": "incomplete provider response",
		})
		return nil, appErr.ErrInvalidBVN
	}

	if record.VerifiedPhone == nil {
		log.Printf("ValidateBVNWithPrembly: verified phone is nil bvn=%s", MaskSub(bvn))
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderPrembly,
			"reason for failure": "verified phone is nil",
		})
		return nil, appErr.ErrInvalidBVN
	}

	bvnRecord := &models.BVNRecord{
		ID:              uuid.NewString(),
		UserID:          "",
		FirstName:       firstName,
		MiddleName:      middleName,
		LastName:        lastName,
		Gender:          firstNonEmptyString(d.Gender, fb.Gender),
		Nationality:     firstNonEmptyString(d.Nationality, fb.Nationality),
		StateOfOrigin:   firstNonEmptyString(d.StateOfOrigin, fb.StateOfOrigin),
		DateOfBirth:     parseBVNRecordDOB(dobField),
		MaritalStatus:   firstNonEmptyString(d.MaritalStatus, fb.MaritalStatus),
		EmailAddress:    firstNonEmptyString(d.Email, fb.Email),
		PassportOnBVN:   firstNonEmptyString(trimmedStringValue(d.Image), trimmedStringValue(fb.Image)),
		FullHomeAddress: firstNonEmptyString(d.ResidentialAddress, fb.ResidentialAddress),
		MobilePhone:     *record.VerifiedPhone,
		BankName:        firstNonEmptyString(d.EnrollmentBank, fb.EnrollmentBank),
		BVN:             strings.TrimSpace(firstNonEmptyString(d.BVN, fb.BVN, bvn)),
	}
	bvnRecord.City = trimmedStringPtr(firstNonEmptyString(d.LGAOfOrigin, fb.LGAOfOrigin))

	if err := s.saveVerifiedBVN(ctx, record, bvnRecord); err != nil {
		log.Printf("ValidateBVNWithPrembly: saveVerifiedBVN failed: %v", err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderPrembly,
			"reason for failure": "failed to persist verification record",
		})
		return nil, err
	}

	maskedPhone, err := phoneUtil.MaskPhone(*record.VerifiedPhone)
	if err != nil {
		log.Printf("ValidateBVNWithPrembly: MaskPhone failed: %v", err)
		s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusFailure, map[string]interface{}{
			"BVN":                maskedBVN,
			"provider":           ProviderPrembly,
			"reason for failure": "failed to mask phone",
		})
		return nil, err
	}

	s.logVerificationAudit(ctx, auditlog.ResourceTypeBVN, "BVN_VALIDATION", verificationID, auditlog.StatusSuccess, map[string]interface{}{
		"BVN":      maskedBVN,
		"provider": ProviderPrembly,
	})

	return &bvnInfo{
		name:           fullName,
		dob:            dobField,
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
		authRepo := NewRespository(txDB, s.repo.cipher)
		verificationRepo := verification.NewVerification(txDB, s.verification.Cipher())

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
