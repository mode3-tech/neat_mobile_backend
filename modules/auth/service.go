package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/database/tx"
	"neat_mobile_app_backend/internal/validators"
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

func (s *AuthService) Register(ctx context.Context, req RegisterRequest, ip string) (*AuthObject, error) {
	var createdUser *models.User
	var err error

	now := time.Now().UTC()

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
			IP:          ip,
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
		UserID:   createdUser.ID,
		SID:      sid,
		DeviceID: &req.Device.DeviceID,
		IP:       &ip,
	}

	if err = s.repo.AddAccessToken(ctx, authSession); err != nil {
		return nil, err
	}

	refreshToken, jti, _, err := s.jwtSigner.IssueRefreshToken(createdUser.ID, sid)
	if err != nil {
		return nil, err
	}

	hashedRefreshToken := sha256.Sum256([]byte(refreshToken))

	refreshTokenObj := &models.RefreshToken{
		JTI:       jti,
		SessionID: sid,
		UserID:    createdUser.ID,
		TokenHash: hex.EncodeToString(hashedRefreshToken[:]),
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour * 24 * 30),
	}

	if err := s.repo.AddRefreshToken(ctx, refreshTokenObj); err != nil {
		return nil, err
	}

	return &AuthObject{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) ValidateNIN(ctx context.Context, bvnVerificationID, nin string) (*ninInfo, error) {
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

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("bvn verification not found")
	}

	if err != nil {
		return nil, err
	}

	if !compareBVNAndNinDetails(*row.VerifiedName, *row.VerifiedDOB, fullName, SerializeDOB(strings.TrimSpace(resp.Data.BirthDate))) {
		return nil, errors.New("bvn and nin do not match")
	}

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
		log.Printf("bvn provider source not configured; falling back to default provider")
		return s.validateBVNWithFallback(ctx, bvn)
	}

	provider, err := s.providerSource.GetCurrentProvider(ctx)
	if err != nil {
		log.Printf("failed to resolve bvn provider from source: %v", err)
		return s.validateBVNWithFallback(ctx, bvn)
	}

	switch provider {
	case ProviderTendar:
		return s.ValidateBVNWithTendar(ctx, bvn)
	case ProviderPrembly:
		return s.ValidateBVNWithPrembly(ctx, bvn)
	default:
		log.Printf("unsupported bvn provider %q from source; falling back to default provider", provider)
		return s.validateBVNWithFallback(ctx, bvn)
	}
}

func (s *AuthService) validateBVNWithFallback(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.tender != nil {
		return s.ValidateBVNWithTendar(ctx, bvn)
	}

	if s.prembly != nil {
		return s.ValidateBVNWithPrembly(ctx, bvn)
	}

	return nil, errors.New("bvn providers are not configured")
}

func (s *AuthService) createUser(ctx context.Context, repo *Repository, req RegisterRequest) (*models.User, error) {
	var isEmailVerified bool
	phoneRecord, err := repo.GetValidationRow(ctx, req.PhoneVerificationID)
	if err != nil {
		return nil, errors.New("phone verification record not found")
	}

	if *phoneRecord.VerifiedPhone != req.PhoneNumber {
		return nil, errors.New("phone number does not match")
	}

	_, err = repo.GetUserByPhone(ctx, req.PhoneNumber)
	if err != nil {
		return nil, errors.New("user already exists")
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

	if err = validators.ValidatePassword(req.Password); err != nil {
		return nil, errors.New(err.Error())
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
	if err != nil {
		return nil, err
	}

	return createdUser, nil
}

func (s *AuthService) Login(ctx context.Context, deviceID, ip, phone, password string) (*LoginInitObject, error) {
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
			sessionToken, sessionErr := s.createPendingDeviceSession(ctx, user.ID, deviceID, ip)
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
		sessionToken, sessionErr := s.createPendingDeviceSession(ctx, user.ID, deviceID, ip)
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

func (s *AuthService) createPendingDeviceSession(ctx context.Context, userID, deviceID, ip string) (string, error) {
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
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.deviceRepo.CreatePendingSession(ctx, row); err != nil {
		return "", err
	}

	return sessionToken, nil
}

func (s *AuthService) VerifyDeviceChallenge(ctx context.Context, challenge, signature, deviceID, ip string) (*AuthObject, error) {
	challenge = strings.TrimSpace(challenge)
	if challenge == "" {
		return nil, errors.New("challenge is required")
	}

	signature = strings.TrimSpace(signature)
	if signature == "" {
		return nil, errors.New("signature is required")
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	if s.deviceRepo == nil {
		return nil, errors.New("device repository not configured")
	}

	challengeHash := sha256.Sum256([]byte(challenge))
	storedChallenge, err := s.deviceRepo.GetChallengeByHash(ctx, hex.EncodeToString(challengeHash[:]))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Println("challenge not found")
			return nil, errors.New("invalid challenge")
		}
		return nil, err
	}

	now := time.Now().UTC()
	if storedChallenge.IsUsed() || storedChallenge.IsExpired(now) {
		fmt.Println("challenge expired")
		return nil, errors.New("invalid challenge")
	}

	if storedChallenge.DeviceID != deviceID {
		fmt.Println("challenge device id don't match")
		return nil, errors.New("invalid challenge")
	}

	deviceRecord, err := s.deviceRepo.FindDevice(ctx, storedChallenge.UserID, storedChallenge.DeviceID)
	fmt.Printf("user id: %s\n", storedChallenge.UserID)
	fmt.Printf("device id: %s\n", storedChallenge.DeviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Println("device not found")
			return nil, errors.New("device verification failed")
		}
		return nil, err
	}

	if !deviceRecord.IsActive || !deviceRecord.IsTrusted {
		return nil, errors.New("device verification failed")
	}

	validSig, err := verifyDeviceSignature(deviceRecord.PublicKey, challenge, signature)
	fmt.Printf("public key: %s\n", deviceRecord.PublicKey)
	fmt.Printf("challenge: %s\n", challenge)
	fmt.Printf("signature: %s\n", signature)
	if err != nil || !validSig {
		fmt.Println("sign failed")
		return nil, errors.New("device verification failed")
	}

	marked, err := s.deviceRepo.MarkChallengeUsed(ctx, storedChallenge.ID, now)
	if err != nil {
		return nil, err
	}
	if !marked {
		return nil, errors.New("invalid challenge")
	}

	if err := s.deviceRepo.UpdateLastUsed(ctx, deviceRecord.UserID, deviceRecord.DeviceID, now); err != nil {
		return nil, err
	}

	return s.issueSessionTokens(ctx, storedChallenge.UserID, deviceRecord.DeviceID, ip)
}

func (s *AuthService) issueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*AuthObject, error) {
	sid := uuid.NewString()

	accessToken, err := s.jwtSigner.IssueAccessToken(userID, sid)
	if err != nil {
		return nil, err
	}

	authSession := &models.AuthSession{
		UserID: userID,
		SID:    sid,
	}
	if trimmedDeviceID := strings.TrimSpace(deviceID); trimmedDeviceID != "" {
		authSession.DeviceID = &trimmedDeviceID
	}
	if trimmedIP := strings.TrimSpace(ip); trimmedIP != "" {
		authSession.IP = &trimmedIP
	}

	if err := s.repo.AddAccessToken(ctx, authSession); err != nil {
		return nil, err
	}

	refreshToken, jti, refreshExpiresAt, err := s.jwtSigner.IssueRefreshToken(userID, sid)
	if err != nil {
		return nil, err
	}

	hashedRefreshToken := sha256.Sum256([]byte(refreshToken))
	if hashedRefreshToken == [32]byte{} {
		return nil, errors.New("error while hashing refresh token")
	}

	now := time.Now().UTC()
	refreshTokenObj := &models.RefreshToken{
		JTI:       jti,
		SessionID: sid,
		UserID:    userID,
		TokenHash: hex.EncodeToString(hashedRefreshToken[:]),
		IssuedAt:  now,
		ExpiresAt: refreshExpiresAt,
	}

	if err := s.repo.AddRefreshToken(ctx, refreshTokenObj); err != nil {
		return nil, err
	}

	return &AuthObject{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func verifyDeviceSignature(publicKeyEncoded, challenge, signatureEncoded string) (bool, error) {
	publicKeyBytes, err := decodeEncodedBytes(publicKeyEncoded, ed25519.PublicKeySize)
	if err != nil {
		return false, err
	}

	signatureBytes, err := decodeEncodedBytes(signatureEncoded, ed25519.SignatureSize)
	if err != nil {
		return false, err
	}

	return ed25519.Verify(ed25519.PublicKey(publicKeyBytes), []byte(challenge), signatureBytes), nil
}

func decodeEncodedBytes(value string, expectedLen int) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("empty value")
	}

	decoders := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
		hex.DecodeString,
	}

	for _, decode := range decoders {
		decoded, err := decode(trimmed)
		if err != nil {
			continue
		}
		if len(decoded) == expectedLen {
			return decoded, nil
		}
	}

	return nil, errors.New("invalid encoded value")
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

func (s *AuthService) RefreshAccessToken(ctx context.Context, deviceID, refreshToken string) (*AuthObject, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, errors.New("invalid refresh token")
	}

	sub, sid, oldJTI, err := s.jwtSigner.ExtractRefreshTokenIdentifiers(refreshToken)

	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	refreshTokenObj, err := s.repo.GetRefreshTokenWithJTI(ctx, oldJTI)

	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	if refreshTokenObj.TokenHash != "" {
		receivedHash := sha256.Sum256([]byte(refreshToken))
		if refreshTokenObj.TokenHash != hex.EncodeToString(receivedHash[:]) {
			return nil, errors.New("invalid refresh token")
		}
	}

	if s.deviceRepo == nil {
		return nil, errors.New("device repository not configured")
	}

	deviceRecord, err := s.deviceRepo.FindDevice(ctx, refreshTokenObj.UserID, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("device not found")
		}
		return nil, err
	}

	if !deviceRecord.IsActive || !deviceRecord.IsTrusted {
		return nil, errors.New("device not allowed")
	}

	now := time.Now().UTC()

	if refreshTokenObj.RevokedAt != nil || refreshTokenObj.SessionID != sid || refreshTokenObj.UserID != sub || refreshTokenObj.ExpiresAt.Before(now) {
		return nil, errors.New("invalid refresh token")
	}

	accessSession, err := s.repo.GetAccessTokenWithSID(ctx, sid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid session")
		}
		return nil, err
	}

	if accessSession.RevokedAt != nil || accessSession.UserID != sub || accessSession.DeviceID == nil || strings.TrimSpace(*accessSession.DeviceID) != deviceID {
		return nil, errors.New("device not allowed")
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
		JTI:       newJTI,
		SessionID: sid,
		UserID:    sub,
		TokenHash: hex.EncodeToString(hashedRefreshToken[:]),
		IssuedAt:  now,
		ExpiresAt: newExpiresAt,
	}

	if err := s.repo.RotateRefreshToken(ctx, oldJTI, newRefreshTokenRow); err != nil {
		return nil, err
	}

	return &AuthObject{AccessToken: accessToken, RefreshToken: newRefreshToken}, nil

}

func (s *AuthService) ValidateBVNWithTendar(ctx context.Context, bvn string) (*bvnInfo, error) {
	if s.tender == nil {
		log.Printf("tendar validator is not configured")
		return nil, errors.New("tendar validator is not configured")
	}

	if bvn == "" {
		log.Printf("bvn is required")
		return nil, errors.New("bvn is required")
	}

	if len(bvn) < 11 || len(bvn) > 11 {
		log.Printf("invalid bvn number")
		return nil, errors.New("invalid bvn number")
	}

	bvnDetails, err := s.tender.ValidateBVNWithTendar(ctx, bvn)
	if err != nil {
		fmt.Printf("service %s\n", err.Error())
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
		log.Printf("tendar verification id: %s\n", providerVerificationID)
		record.ProviderVerificationID = &providerVerificationID
	}
	if fullName != "" {
		record.VerifiedName = &fullName
	}
	if phone := strings.TrimSpace(bvnDetails.Data.Details.PhoneNumber); phone != "" {
		record.VerifiedPhone = &phone
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

	if err := s.verification.AddVerification(ctx, record); err != nil {
		log.Printf("failed to add verification record err=%v", err)
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
