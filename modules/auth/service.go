package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"neat_mobile_app_backend/internal/database/tx"
	"neat_mobile_app_backend/internal/notify"
	"neat_mobile_app_backend/internal/timeutil"
	"neat_mobile_app_backend/internal/validators"
	"neat_mobile_app_backend/models"
	authotp "neat_mobile_app_backend/modules/auth/otp"
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
	smsSender      notify.SMSSender
	otpPepper      string
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

const (
	loginOTPPurpose = authotp.PurposeLogin
	loginOTPChannel = authotp.ChannelSMS
)

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

func (s *AuthService) ConfigureLoginOTP(smsSender notify.SMSSender, pepper string) {
	s.smsSender = smsSender
	s.otpPepper = strings.TrimSpace(pepper)
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

func (s *AuthService) ValidateNIN(ctx context.Context, ninVerificationID, nin string) (*ninInfo, error) {
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

	row, err := s.repo.GetValidationRow(ctx, ninVerificationID)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("nin verification not found")
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
		VerifiedID:    &nin,
		VerifiedName:  &fullName,
		VerifiedPhone: &resp.Data.TelephoneNo,
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

func (s *AuthService) ValidateBVN(ctx context.Context, bvn string) (*bvnInfo, error) {
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

func (s *AuthService) createUser(ctx context.Context, repo *Repository, req RegisterRequest) (*models.User, error) {
	var isEmailVerified bool
	phoneRecord, err := repo.GetValidationRow(ctx, req.PhoneVerificationID)
	if err != nil {
		return nil, errors.New("phone verification record not found")
	}

	normalizedNumber, err := NormalizeNigerianNumber(req.PhoneNumber)
	if err != nil {
		return nil, errors.New(err.Error())
	}

	if *phoneRecord.VerifiedPhone != normalizedNumber {
		return nil, errors.New("phone number does not match")
	}

	existingUser, err := repo.GetUserByPhone(ctx, normalizedNumber)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existingUser != nil {
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

	dob, err := timeutil.ParseDOB(*ninRecord.VerifiedDOB)

	if err != nil {
		return nil, errors.New(err.Error())
	}

	user := &models.User{
		ID:                  uuid.NewString(),
		Phone:               normalizedPhone,
		Email:               req.Email,
		PasswordHash:        hashedPassword,
		PinHash:             hashedTransactionPin,
		IsEmailVerified:     isEmailVerified,
		BVN:                 *bvnRecord.VerifiedID,
		NIN:                 *ninRecord.VerifiedID,
		DOB:                 dob,
		IsPhoneVerified:     true,
		IsBvnVerified:       true,
		IsNinVerified:       true,
		IsBiometricsEnabled: req.IsBiometricsEnabled,
	}

	createdUser, err := repo.CreateUser(ctx, user)
	if err != nil {
		fmt.Println("created user" + " " + err.Error())
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
			return s.startNewDeviceFlow(ctx, user.ID, user.Phone, deviceID, ip)
		}
		return nil, err
	}

	if !deviceRecord.IsActive || !deviceRecord.IsTrusted {
		return s.startNewDeviceFlow(ctx, user.ID, user.Phone, deviceID, ip)
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

func (s *AuthService) VerifyNewDevice(ctx context.Context, ip string, req NewDeviceResquest) (*AuthObject, error) {
	if s.tx == nil {
		return nil, errors.New("transaction manager not configured")
	}
	if s.deviceRepo == nil {
		return nil, errors.New("device repository not configured")
	}
	if strings.TrimSpace(s.otpPepper) == "" {
		return nil, errors.New("otp pepper not configured")
	}

	sessionToken := strings.TrimSpace(req.SessionToken)
	if sessionToken == "" {
		return nil, errors.New("session token is required")
	}
	if strings.TrimSpace(req.OTP) == "" {
		return nil, errors.New("otp is required")
	}

	deviceID := strings.TrimSpace(req.Device.DeviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}
	if strings.TrimSpace(req.Device.PublicKey) == "" {
		return nil, errors.New("public key is required")
	}

	var authObj *AuthObject

	err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		deviceRepo := device.NewDeviceRepository(txDB)
		otpRepo := authotp.NewOTPRepository(txDB)
		authRepo := NewRespository(txDB)

		sessionTokenHash := sha256.Sum256([]byte(sessionToken))
		hashedSessionToken := hex.EncodeToString(sessionTokenHash[:])

		pendingSession, err := deviceRepo.GetPendingSessionByHash(ctx, hashedSessionToken)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("invalid session token")
			}
			return err
		}

		now := time.Now().UTC()
		if pendingSession.IsUsed() || pendingSession.IsExpired(now) {
			return errors.New("invalid session token")
		}
		if strings.TrimSpace(pendingSession.DeviceID) != deviceID {
			return errors.New("invalid session token")
		}
		if strings.TrimSpace(pendingSession.OTPRef) == "" {
			return errors.New("invalid session token")
		}

		activeOTP, err := otpRepo.GetActiveOTPByID(ctx, strings.TrimSpace(pendingSession.OTPRef), loginOTPPurpose)
		if err != nil {
			return err
		}
		if activeOTP == nil {
			return errors.New("invalid otp")
		}

		maxAttempts := activeOTP.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		if activeOTP.AttemptCount >= maxAttempts {
			return errors.New("invalid otp")
		}

		hashedOTP, err := authotp.HashOTP(s.otpPepper, loginOTPPurpose, activeOTP.Destination, strings.TrimSpace(req.OTP))
		if err != nil || !authotp.HashEqualHex(hashedOTP, activeOTP.OTPHash) {
			if updateErr := otpRepo.IncrementAttempt(ctx, activeOTP.ID); updateErr != nil {
				return updateErr
			}
			return errors.New("invalid otp")
		}

		if err := otpRepo.ConsumeOTP(ctx, activeOTP.ID, now); err != nil {
			return err
		}

		deviceRow := &device.UserDevice{
			ID:          deviceID,
			UserID:      pendingSession.UserID,
			DeviceID:    deviceID,
			PublicKey:   strings.TrimSpace(req.Device.PublicKey),
			DeviceName:  strings.TrimSpace(req.Device.DeviceName),
			DeviceModel: strings.TrimSpace(req.Device.DeviceModel),
			OS:          strings.TrimSpace(req.Device.OS),
			OSVersion:   strings.TrimSpace(req.Device.OSVersion),
			AppVersion:  strings.TrimSpace(req.Device.AppVersion),
			IP:          ip,
			LastUsedAt:  now,
		}
		if err := deviceRepo.UpsertDevicePublicKey(ctx, deviceRow); err != nil {
			return err
		}
		if err := deviceRepo.ActivateAndTrustDevice(ctx, pendingSession.UserID, deviceID, now, ip); err != nil {
			return err
		}

		marked, err := deviceRepo.MarkPendingSessionUsed(ctx, pendingSession.ID, now)
		if err != nil {
			return err
		}
		if !marked {
			return errors.New("invalid session token")
		}

		authObj, err = s.issueSessionTokensWithRepo(ctx, authRepo, pendingSession.UserID, deviceID, ip)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return authObj, nil
}

func (s *AuthService) startNewDeviceFlow(ctx context.Context, userID, phone, deviceID, ip string) (*LoginInitObject, error) {
	if s.tx == nil {
		return nil, errors.New("transaction manager not configured")
	}
	if s.smsSender == nil {
		return nil, errors.New("sms sender not configured")
	}
	if strings.TrimSpace(s.otpPepper) == "" {
		return nil, errors.New("otp pepper not configured")
	}

	normalizedPhone, err := NormalizeNigerianNumber(phone)
	if err != nil {
		return nil, err
	}

	var sessionToken string
	var generatedOTP string

	err = s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		deviceRepo := device.NewDeviceRepository(txDB)

		otpRef, code, err := s.upsertNewDeviceLoginOTP(ctx, txDB, userID, normalizedPhone)
		if err != nil {
			return err
		}
		generatedOTP = code

		token, err := s.createPendingDeviceSession(ctx, deviceRepo, userID, deviceID, ip, otpRef)
		if err != nil {
			return err
		}
		sessionToken = token

		otpMessage := fmt.Sprintf("Your login OTP is %s. It expires in 10 minutes. Do not share this code with anyone.", generatedOTP)
		if err := s.smsSender.Send(ctx, normalizedPhone, otpMessage); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &LoginInitObject{
		Status:       LoginStatusNewDeviceDetected,
		SessionToken: sessionToken,
	}, nil
}

func (s *AuthService) upsertNewDeviceLoginOTP(ctx context.Context, txDB *gorm.DB, userID, normalizedPhone string) (string, string, error) {
	if txDB == nil {
		return "", "", errors.New("transaction db not configured")
	}

	now := time.Now().UTC()
	const ttl = 10 * time.Minute
	const cooldown = 30 * time.Second

	otpRepo := authotp.NewOTPRepository(txDB)
	activeOTP, err := otpRepo.GetActiveOTP(ctx, normalizedPhone, loginOTPPurpose)
	if err != nil {
		return "", "", err
	}

	if activeOTP != nil {
		maxResends := activeOTP.MaxResends
		if maxResends <= 0 {
			maxResends = 3
		}

		if activeOTP.NextSendAt != nil && now.Before(*activeOTP.NextSendAt) {
			return "", "", errors.New("too many requests")
		}
		if activeOTP.ResendCount >= maxResends {
			return "", "", errors.New("too many requests")
		}
	}

	generatedOTP, err := authotp.Generate6DigitOTP()
	if err != nil {
		return "", "", errors.New("unable to generate OTP")
	}

	hashedOTP, err := authotp.HashOTP(s.otpPepper, loginOTPPurpose, normalizedPhone, generatedOTP)
	if err != nil {
		return "", "", errors.New("unable to hash OTP")
	}

	expiresAt := now.Add(ttl)
	nextSendAt := now.Add(cooldown)

	if activeOTP == nil {
		otpRow := &authotp.OTPModel{
			ID:           uuid.NewString(),
			UserID:       userID,
			Purpose:      loginOTPPurpose,
			Channel:      loginOTPChannel,
			Destination:  normalizedPhone,
			OTPHash:      hashedOTP,
			ExpiresAt:    expiresAt,
			NextSendAt:   &nextSendAt,
			ResendCount:  0,
			MaxResends:   3,
			AttemptCount: 0,
			MaxAttempts:  5,
			IssuedAt:     now,
		}
		if err := otpRepo.CreateOTP(ctx, otpRow); err != nil {
			return "", "", err
		}
		return otpRow.ID, generatedOTP, nil
	}

	if err := otpRepo.UpdateForResend(ctx, activeOTP.ID, hashedOTP, expiresAt, nextSendAt); err != nil {
		return "", "", err
	}

	return activeOTP.ID, generatedOTP, nil
}

func (s *AuthService) createPendingDeviceSession(ctx context.Context, deviceRepo *device.DeviceRepository, userID, deviceID, ip, otpRef string) (string, error) {
	repo := deviceRepo
	if repo == nil {
		repo = s.deviceRepo
	}
	if repo == nil {
		return "", errors.New("device repository not configured")
	}
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
		OTPRef:           strings.TrimSpace(otpRef),
		ExpiresAt:        now.Add(10 * time.Minute),
		IP:               ip,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := repo.CreatePendingSession(ctx, row); err != nil {
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

func (s *AuthService) ResendNewDeviceOTP(ctx context.Context, req ResendNewDeviceOTPRequest) error {
	if s.tx == nil {
		return errors.New("transaction manager not configured")
	}
	if s.smsSender == nil {
		return errors.New("sms sender not configured")
	}
	if strings.TrimSpace(s.otpPepper) == "" {
		return errors.New("otp pepper not configured")
	}

	req.SessionToken = strings.TrimSpace(req.SessionToken)
	if req.SessionToken == "" {
		return errors.New("session token is required")
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		return errors.New("device id is required")
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		deviceRepo := device.NewDeviceRepository(txDB)
		authRepo := NewRespository(txDB)

		sum := sha256.Sum256([]byte(req.SessionToken))
		session, err := deviceRepo.GetPendingSessionByHash(ctx, hex.EncodeToString(sum[:]))
		if err != nil {
			return errors.New("invalid session token")
		}

		now := time.Now().UTC()
		if session.IsUsed() || session.IsExpired(now) || strings.TrimSpace(session.DeviceID) != req.DeviceID {
			return errors.New("invalid session token")
		}

		user, err := authRepo.GetUserByID(ctx, session.UserID)
		if err != nil {
			return err
		}

		phone, err := NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return err
		}

		otpRef, code, err := s.upsertNewDeviceLoginOTP(ctx, txDB, session.UserID, phone)
		if err != nil {
			return err
		}

		if err := deviceRepo.RefreshPendingSession(ctx, session.ID, otpRef, now.Add(10*time.Minute), now); err != nil {
			return err
		}

		return s.smsSender.Send(ctx, phone, fmt.Sprintf("Your login OTP is %s. It expires in 10 minutes. Do not share this code with anyone.", code))
	})
}

func (s *AuthService) issueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*AuthObject, error) {
	return s.issueSessionTokensWithRepo(ctx, s.repo, userID, deviceID, ip)
}

func (s *AuthService) issueSessionTokensWithRepo(ctx context.Context, repo *Repository, userID, deviceID, ip string) (*AuthObject, error) {
	if repo == nil {
		return nil, errors.New("auth repository not configured")
	}

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

	if err := repo.AddAccessToken(ctx, authSession); err != nil {
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

	if err := repo.AddRefreshToken(ctx, refreshTokenObj); err != nil {
		return nil, err
	}

	return &AuthObject{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func verifyDeviceSignature(publicKeyEncoded, challenge, signatureEncoded string) (bool, error) {
	publicKeyBytes, err := decodeEncodedBytesAny(publicKeyEncoded)
	if err != nil {
		return false, errors.New("invalid public key encoding")
	}

	signatureBytes, err := decodeEncodedBytesAny(signatureEncoded)
	if err != nil {
		return false, errors.New("invalid signature enconding")
	}

	pub, err := parseP256PublicKey(publicKeyBytes)
	if err != nil {
		return false, err
	}

	digest := sha256.Sum256([]byte(challenge))

	// preferred: ASN.1 DER signature
	if ecdsa.VerifyASN1(pub, digest[:], signatureBytes) {
		return true, nil
	}

	// optional fallback: raw R||S (64 bytes)
	if len(signatureBytes) == 64 {
		r := new(big.Int).SetBytes(signatureBytes[:32])
		s := new(big.Int).SetBytes(signatureBytes[32:])
		return ecdsa.Verify(pub, digest[:], r, s), nil
	}

	return false, nil
}

func parseP256PublicKey(b []byte) (*ecdsa.PublicKey, error) {
	if block, _ := pem.Decode(b); block != nil {
		b = block.Bytes
	}

	if anyKey, err := x509.ParsePKIXPublicKey(b); err == nil {
		pub, ok := anyKey.(*ecdsa.PublicKey)
		if !ok || pub.Curve != elliptic.P256() {
			return nil, errors.New("public key is not ECDSA P-256")
		}
		return pub, nil
	}

	// uncompressed EC point: 65 bytes, starts with 0x04
	if len(b) == 65 && b[0] == 0x04 {
		x, y := elliptic.Unmarshal(elliptic.P256(), b)
		if x == nil {
			return nil, errors.New("invalid P-256 public key point")
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
	}

	return nil, errors.New("unsupported public key format")
}

func decodeEncodedBytesAny(value string) ([]byte, error) {
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
		if b, err := decode(trimmed); err == nil && len(b) > 0 {
			return b, nil
		}
	}

	return nil, errors.New("invalid encoded value")
}

func generate6DigitOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", errors.New("error generating OTP")
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
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

	if len(bvn) != 11 {
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

func (s *AuthService) ForgotPassword(ctx context.Context, req ForgotPasswordRequest, deviceID string) error {

	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}

	phone, err := NormalizeNigerianNumber(strings.TrimSpace(req.Phone))

	if err != nil {
		return errors.New(err.Error())
	}

	user, err := s.repo.GetUserByPhone(ctx, phone)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || user == nil {
			return errors.New("no account exists under this phone number")
		}
		return err
	}

	_, err = s.deviceRepo.FindDevice(ctx, user.ID, deviceID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("no record of device found")
		}
		return err
	}

	generatedOTP, err := generate6DigitOTP()

	if err != nil {
		return errors.New(err.Error())
	}

	otpByte := sha256.Sum256([]byte(generatedOTP))
	otpHash := hex.EncodeToString(otpByte[:])

	otpRepo := authotp.NewOTPRepository(s.repo.db)

	now := time.Now().UTC()

	otp := &authotp.OTPModel{
		ID:          uuid.NewString(),
		UserID:      user.ID,
		Purpose:     authotp.PurposePasswordReset,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		OTPHash:     otpHash,
		ExpiresAt:   now.Add(10 * time.Minute),
	}

	if err := otpRepo.CreateOTP(ctx, otp); err != nil {
		return errors.New("error occured while saving otp")
	}

	s.smsSender.Send(ctx, phone, string(fmt.Sprintf("Your password reset code is %s. It expires in 10 minutes. Do not share this code with anyone.", generatedOTP)))

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, req ResetPasswordRequest, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("device id is required")
	}

	resetCode := strings.TrimSpace(req.ResetCode)
	if resetCode == "" {
		return errors.New("reset code is required")
	}
	if len(resetCode) != 6 {
		return errors.New("invalid reset code")
	}

	password := strings.TrimSpace(req.Password)
	if err := validators.ValidatePassword(password); err != nil {
		return errors.New(err.Error())
	}

	hashedPassword, err := HashPassword(password)
	if err != nil {
		return err
	}

	if s.tx == nil {
		return errors.New("transaction manager not configured")
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		otpRepo := authotp.NewOTPRepository(txDB)

		var boundDevice device.UserDevice
		if err := txDB.WithContext(ctx).
			Where("device_id = ?", deviceID).
			Order("last_used_at DESC").
			First(&boundDevice).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("invalid device id")
			}
			return err
		}

		var user models.User
		if err := txDB.WithContext(ctx).
			Table("wallet_users").
			Select("id, phone").
			Where("id = ?", boundDevice.UserID).
			First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("invalid device id")
			}
			return err
		}

		normalizedPhone, err := NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return errors.New("invalid device id")
		}

		activeOTP, err := otpRepo.GetActiveOTP(ctx, normalizedPhone, authotp.PurposePasswordReset)
		if err != nil {
			return err
		}
		if activeOTP == nil {
			return errors.New("invalid reset code")
		}

		maxAttempts := activeOTP.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		if activeOTP.AttemptCount >= maxAttempts {
			return errors.New("invalid reset code")
		}

		providedResetCodeHash := sha256.Sum256([]byte(resetCode))
		if !authotp.HashEqualHex(hex.EncodeToString(providedResetCodeHash[:]), activeOTP.OTPHash) {
			if err := otpRepo.IncrementAttempt(ctx, activeOTP.ID); err != nil {
				return err
			}
			return errors.New("invalid reset code")
		}

		now := time.Now().UTC()
		if err := otpRepo.ConsumeOTP(ctx, activeOTP.ID, now); err != nil {
			return err
		}

		result := txDB.WithContext(ctx).
			Model(&models.User{}).
			Where("id = ?", user.ID).
			Update("password_hash", hashedPassword)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("no account exists under this phone number")
		}

		return nil
	})
}
