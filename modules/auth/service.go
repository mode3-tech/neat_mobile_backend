package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"neat_mobile_app_backend/internal/database/tx"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/auth/verification"
	"neat_mobile_app_backend/modules/device"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

type AuthService struct {
	repo           *Repository
	verification   *verification.VerificationRepo
	tx             *tx.Transactor
	deviceRepo     *device.DeviceRepository
	jwtSigner      JWTSigner
	tender         TendarValidation
	prembly        PremblyValidation
	nin            NINValidation
	providerSource BVNProviderSource
}

type bvnInfo struct {
	name           string
	dob            string
	phone          string
	verificationID string
}

type ninInfo struct {
	name           string
	dob            string
	phone          string
	verificationID string
}

func NewAuthService(repo *Repository, verification *verification.VerificationRepo, tx *tx.Transactor, deviceRepo *device.DeviceRepository, signer JWTSigner, tender TendarValidation, prembly PremblyValidation, nin NINValidation, providerSource BVNProviderSource) *AuthService {
	return &AuthService{
		repo:           repo,
		verification:   verification,
		tx:             tx,
		jwtSigner:      signer,
		deviceRepo:     deviceRepo,
		tender:         tender,
		prembly:        prembly,
		nin:            nin,
		providerSource: providerSource,
	}
}

func (s *AuthService) ValidateNIN(ctx context.Context, nin string) (*ninInfo, error) {
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

	verificationID := uuid.NewString()
	subjectHashBytes := sha256.Sum256([]byte(strings.TrimSpace(nin)))
	subjectHash := hex.EncodeToString(subjectHashBytes[:])
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	maskedNIN := MaskSub(nin)

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

func (s *AuthService) ValidateBVN(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.providerSource == nil {
		return nil, errors.New("something went wrong")
	}

	provider, err := s.providerSource.GetCurrentProvider(ctx)
	if err != nil {
		return s.ValidateBVNWithTendar(ctx, bvn)
	}

	switch provider {
	case ProviderTendar:
		return s.ValidateBVNWithTendar(ctx, bvn)
	case ProviderPrembly:
		return s.ValidateBVNWithPrembly(ctx, bvn)
	default:
		return nil, fmt.Errorf("unsupported bvn provider %q", provider)
	}
}

func (s *AuthService) createUser(ctx context.Context, repo *Repository, req RegisterRequest) (*models.User, error) {
	var isEmailVerified bool
	phoneRecord, err := repo.GetValidationRow(ctx, req.PhoneVerificationID)
	if err != nil {
		return nil, errors.New("phone verification record not found")
	}

	bvnRecord, err := repo.GetValidationRow(ctx, req.BVNVerificationID)
	if err != nil || bvnRecord.VerifiedName == nil || bvnRecord.VerifiedDOB == nil {
		return nil, errors.New("bvn verification record not found")
	}

	ninRecord, err := repo.GetValidationRow(ctx, req.NINVerificationID)
	if err != nil || ninRecord.VerifiedName == nil || ninRecord.VerifiedDOB == nil {
		return nil, errors.New("nin verification record not found")
	}

	if req.Email != "" {
		emailRecord, err := repo.GetValidationRow(ctx, req.EmailVerificationID)
		if err != nil || emailRecord.VerifiedName == nil || emailRecord.VerifiedDOB == nil {
			return nil, errors.New("email verification record not found")
		}

		if emailRecord.VerifiedName != phoneRecord.VerifiedName || emailRecord.VerifiedDOB != phoneRecord.VerifiedDOB {
			return nil, errors.New("unable to confirm email and phone number belong to the same person due to names or date of births mismatch")
		}

		isEmailVerified = true
	}

	bvnName := strings.ToLower(strings.Join(strings.Fields(*bvnRecord.VerifiedName), " "))
	ninName := strings.ToLower(strings.Join(strings.Fields(*ninRecord.VerifiedName), " "))

	if bvnName != ninName || SerializeDOB(*bvnRecord.VerifiedDOB) != SerializeDOB(*ninRecord.VerifiedDOB) {
		return nil, errors.New("unable to confirm bvn and nin belong to the same person due to names or date of births mismatch")
	}

	if req.Password != req.ConfirmPassword {
		return nil, errors.New("passwords do not match")
	}

	if req.TransactionPin != req.ConfirmTransactionPin {
		return nil, errors.New("transaction pins do not match")
	}

	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	hashedTransactionPin, err := HashPassword(req.TransactionPin)
	if err != nil {
		return nil, err
	}

	normalizedPhone, err := NormalizeNigerianNumber(req.PhoneNumber)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		ID:              uuid.NewString(),
		Phone:           normalizedPhone,
		Email:           req.Email,
		PasswordHash:    hashedPassword,
		PinHash:         hashedTransactionPin,
		IsEmailVerified: isEmailVerified,
		IsPhoneVerified: true,
		IsBvnVerified:   true,
		IsNinVerified:   true,
	}

	createdUser, err := repo.CreateUser(ctx, user)

	return createdUser, nil
}

func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*AuthObject, error) {
	var createdUser *models.User
	var err error

	err = s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		authRepo := NewRespository(txDB)
		deviceRepo := device.NewDeviceRepository(txDB)

		createdUser, err = s.createUser(ctx, authRepo, req)
		if err != nil {
			return err
		}

		deviceReq := device.DeviceBindingRequest{
			DeviceID:    req.Device.DeviceID,
			PublicKey:   req.Device.PublicKey,
			DeviceName:  req.Device.DeviceName,
			DeviceModel: req.Device.DeviceModel,
			OS:          req.Device.OS,
			OSVersion:   req.Device.OSVersion,
			AppVersion:  req.Device.AppVersion,
		}
		deviceService := device.NewDeviceService(*deviceRepo)
		return deviceService.BindDevice(ctx, createdUser.ID, &deviceReq)
	})

	if err != nil {
		return nil, err
	}

	sid := uuid.NewString()

	accessToken, err := s.jwtSigner.IssueAccessToken(createdUser.ID, sid)
	if err != nil {
		return nil, err
	}

	authSession := &models.AuthSession{
		UserID:    createdUser.ID,
		SID:       sid,
		DeviceID:  &req.Device.DeviceID,
		IP:        &req.Device.IP,
		UserAgent: &req.Device.UserAgent,
	}

	if err := s.repo.AddAccessToken(ctx, authSession); err != nil {
		return nil, err
	}

	refreshToken, jti, _, err := s.jwtSigner.IssueRefreshToken(createdUser.ID, sid)
	if err != nil {
		return nil, err
	}

	hashedRefreshToken := sha256.Sum256([]byte(refreshToken))
	if hashedRefreshToken == [32]byte{} {
		return nil, errors.New("error while hashing refresh token")
	}

	refreshTokenObj := &models.RefreshToken{
		JTI:       jti,
		SessionID: sid,
		UserID:    createdUser.ID,
		TokenHash: hex.EncodeToString(hashedRefreshToken[:]),
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour * 24 * 30),
	}

	if err := s.repo.AddRefreshToken(ctx, refreshTokenObj); err != nil {
		return nil, err
	}

	return &AuthObject{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, deviceID, ip, userAgent, phone, password string) (*LoginInitObject, error) {
	normalizedPhone, err := NormalizeNigerianNumber(phone)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByPhone(ctx, normalizedPhone)

	if err != nil {
		fmt.Println("no account exists with this phone")
		return nil, errors.New("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)

	if err != nil {
		fmt.Println("incorrect password")
		return nil, errors.New("invalid credentials")
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	if s.deviceRepo == nil {
		return nil, errors.New("device repository not configured")
	}

	deviceRecord, err := s.deviceRepo.FindDevice(ctx, user.ID, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sessionToken, sessionErr := s.createPendingDeviceSession(ctx, user.ID, deviceID, ip, userAgent)
			if sessionErr != nil {
				return nil, sessionErr
			}

			return &LoginInitObject{
				Status:       LoginStatusNewDeviceDetected,
				SessionToken: sessionToken,
			}, nil
		}

		return nil, err
	}

	if !deviceRecord.IsActive || !deviceRecord.IsTrusted {
		sessionToken, sessionErr := s.createPendingDeviceSession(ctx, user.ID, deviceID, ip, userAgent)
		if sessionErr != nil {
			return nil, sessionErr
		}

		return &LoginInitObject{
			Status:       LoginStatusNewDeviceDetected,
			SessionToken: sessionToken,
		}, nil
	}

	deviceService := device.NewDeviceService(*s.deviceRepo)
	challenge, err := deviceService.CreateChallenge(ctx, user.ID, deviceID)
	if err != nil {
		return nil, err
	}

	return &LoginInitObject{
		Status:    LoginStatusChallengeRequired,
		Challenge: challenge,
	}, nil
}

func (s *AuthService) createPendingDeviceSession(ctx context.Context, userID, deviceID, ip, userAgent string) (string, error) {
	sessionToken, err := randomToken(32)
	if err != nil {
		return "", err
	}

	tokenHash := sha256.Sum256([]byte(sessionToken))
	now := time.Now().UTC()

	row := &models.PendingDeviceSession{
		UserID:           userID,
		DeviceID:         deviceID,
		SessionTokenHash: hex.EncodeToString(tokenHash[:]),
		ExpiresAt:        now.Add(10 * time.Minute),
		IP:               ip,
		UserAgent:        userAgent,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.deviceRepo.CreatePendingSession(ctx, row); err != nil {
		return "", err
	}

	return sessionToken, nil
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("invalid token size")
	}

	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken, accessToken string) error {
	isValidAccessToken := s.jwtSigner.ValidAccessToken(accessToken)
	isValidRefreshToken := s.jwtSigner.ValidRefreshToken(refreshToken)
	if !isValidAccessToken || !isValidRefreshToken {
		return errors.New("invalid access or refresh token")
	}

	accessTokenSub, accessTokenSID, err := s.jwtSigner.ExtractAccessTokenIdentifiers(accessToken)
	if err != nil {
		return err
	}

	refreshTokenSub, refreshTokenSID, jti, err := s.jwtSigner.ExtractRefreshTokenIdentifiers(refreshToken)
	if err != nil {
		return err
	}

	if accessTokenSub != refreshTokenSub {
		return errors.New("access token and refresh token do not match")
	}

	if accessTokenSID != refreshTokenSID {
		return errors.New("access token and refresh token do not match")
	}

	if err = s.repo.DeleteAccessToken(ctx, accessTokenSID); err != nil {
		return err
	}

	if err := s.repo.DeleteRefreshToken(ctx, jti); err != nil {
		return err
	}

	return nil
}

func (s *AuthService) RefreshAccessToken(ctx context.Context, refreshToken string) (*AuthObject, error) {
	sub, sid, oldJTI, err := s.jwtSigner.ExtractRefreshTokenIdentifiers(refreshToken)

	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	refreshTokenObj, err := s.repo.GetRefreshTokenWithJTI(ctx, oldJTI)

	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	now := time.Now().UTC()

	if refreshTokenObj.RevokedAt != nil || refreshTokenObj.SessionID != sid || refreshTokenObj.UserID != sub || refreshTokenObj.ExpiresAt.Before(now) {
		return nil, errors.New("invalid refresh token")
	}

	accessToken, err := s.jwtSigner.IssueAccessToken(sub, sid)
	if err != nil {
		return nil, err
	}

	newRefreshToken, newJTI, newExpiresAt, err := s.jwtSigner.IssueRefreshToken(sub, sid)
	if err != nil || newRefreshToken == "" || newJTI == "" {
		return nil, errors.New("failed to issue refresh token")
	}

	hashedRefreshToken := sha256.Sum256([]byte(newRefreshToken))
	if hashedRefreshToken == [32]byte{} {
		return nil, errors.New("error hashing new refresh token")
	}

	newRefreshTokenRow := &models.RefreshToken{
		JTI:           newJTI,
		SessionID:     sid,
		UserID:        sub,
		TokenHash:     hex.EncodeToString(hashedRefreshToken[:]),
		ReplacedByJTI: &newJTI,
		IssuedAt:      now,
		ExpiresAt:     newExpiresAt,
	}

	if err := s.repo.RotateRefreshToken(ctx, oldJTI, newRefreshTokenRow); err != nil {
		return nil, err
	}

	return &AuthObject{AccessToken: accessToken, RefreshToken: newRefreshToken}, nil

}

func (s *AuthService) ValidateBVNWithTendar(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.tender == nil {
		return nil, errors.New("tendar validator is not configured")
	}

	if bvn == "" {
		return nil, errors.New("bvn is required")
	}

	if len(bvn) < 11 || len(bvn) > 11 {
		return nil, errors.New("invalid bvn number")
	}

	bvnDetails, err := s.tender.ValidateBVNWithTendar(ctx, bvn)
	if err != nil {
		fmt.Printf("service %s\n", err.Error())
		return nil, err
	}
	if bvnDetails == nil {
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
		Provider:      string(ProviderPrembly),
		Status:        models.VerificationStatusVerified,
		SubjectHash:   subjectHash,
		SubjectMasked: &maskedBVN,
		VerifiedAt:    &now,
		ExpiresAt:     &expiresAt,
		VerifiedName:  &fullName,
		VerifiedDOB:   &bvnDetails.Data.Details.DateOfBirth,
	}

	if providerVerificationID := strings.TrimSpace(bvnDetails.VerificationID); providerVerificationID != "" {
		record.ProviderVerificationID = &providerVerificationID
	}
	if fullName != "" {
		record.VerifiedName = &fullName
	}
	if phone := strings.TrimSpace(bvnDetails.Data.PhoneNumber); phone != "" {
		record.VerifiedPhone = &phone
	}
	if email := strings.TrimSpace(bvnDetails.Data.Email); email != "" {
		record.VerifiedEmail = &email
	}
	if dob := strings.TrimSpace(bvnDetails.Data.Details.DateOfBirth); dob != "" {
		record.VerifiedDOB = &dob
	}

	if fullName == "" || bvnDetails.Data.Details.DateOfBirth == "" || bvnDetails.Data.Details.PhoneNumber == "" {
		return nil, errors.New("invalid bvn number")
	}

	if err := s.verification.AddVerification(ctx, record); err != nil {
		return nil, err
	}

	fmt.Printf("tendar verification id: %s\n", verificationID)

	return &bvnInfo{
		name:           fullName,
		dob:            bvnDetails.Data.Details.DateOfBirth,
		phone:          bvnDetails.Data.Details.PhoneNumber,
		verificationID: verificationID,
	}, nil
}

func (s *AuthService) ValidateBVNWithPrembly(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.prembly == nil {
		return nil, errors.New("couldn't resolve prembly provider")
	}

	if bvn == "" {
		return nil, errors.New("bvn is required")
	}

	if len(bvn) < 11 || len(bvn) > 11 {
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
		record.VerifiedPhone = &phone
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

	if err := s.verification.AddVerification(ctx, record); err != nil {
		return nil, err
	}

	fmt.Printf("prembly verification id: %s\n", verificationID)

	return &bvnInfo{
		name:           fullName,
		dob:            bvnDetails.Data.DateOfBirth,
		phone:          bvnDetails.Data.PhoneNumber,
		verificationID: verificationID,
	}, nil
}
