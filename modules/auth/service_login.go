package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	appErr "neat_mobile_app_backend/internal/errors"
	phoneutil "neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/models"
	authotp "neat_mobile_app_backend/modules/auth/otp"
	"neat_mobile_app_backend/modules/device"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (s *Service) Login(ctx context.Context, deviceID, ip, phone, password string) (*LoginInitObject, error) {
	normalizedPhone, err := phoneutil.NormalizeNigerianNumber(phone)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByPhone(ctx, normalizedPhone)

	if err != nil {
		return nil, appErr.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)

	if err != nil {
		return nil, appErr.ErrInvalidCredentials
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, appErr.ErrMissingDeviceID
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

	deviceService := device.NewService(*s.deviceRepo)
	challenge, err := deviceService.CreateChallenge(ctx, user.ID, deviceID, 0)
	if err != nil {
		return nil, err
	}

	return &LoginInitObject{
		Status:    LoginStatusChallengeRequired,
		Challenge: challenge,
	}, nil
}

func (s *Service) CreateChallenge(ctx context.Context, refreshToken, deviceID string) (*ChallengeRequestResponse, error) {
	sub, _, jti, err := s.jwtSigner.ExtractRefreshTokenIdentifiers(refreshToken)
	if err != nil {
		return nil, appErr.ErrDeviceNotAllowed
	}

	tokenRow, err := s.repo.GetRefreshTokenWithJTI(ctx, jti)
	if err != nil {
		return nil, appErr.ErrDeviceNotAllowed
	}

	receivedHash := sha256.Sum256([]byte(refreshToken))
	if tokenRow.TokenHash != hex.EncodeToString(receivedHash[:]) {
		return nil, appErr.ErrDeviceNotAllowed
	}

	if tokenRow.RevokedAt != nil {
		return nil, appErr.ErrDeviceNotAllowed
	}

	if time.Now().UTC().After(tokenRow.ExpiresAt) {
		return nil, appErr.ErrDeviceNotAllowed
	}

	mobileUserID := sub

	_, err = s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return nil, err
	}

	deviceService := device.NewService(*s.deviceRepo)
	challenge, err := deviceService.CreateChallenge(ctx, mobileUserID, deviceID, 60*time.Second)
	if err != nil {
		return nil, err
	}

	return &ChallengeRequestResponse{
		Status:    LoginStatusChallengeRequired,
		Message:   "challenge created successfully",
		Challenge: challenge,
		ExpiresAt: time.Now().UTC().Add(60 * time.Second),
	}, nil
}

func (s *Service) VerifyNewDevice(ctx context.Context, ip string, req NewDeviceResquest) (*VerifiedDeviceResponse, error) {
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
	otp := strings.TrimSpace(req.OTP)
	deviceID := strings.TrimSpace(req.Device.DeviceID)

	var authObj *VerifiedDeviceResponse

	err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		deviceRepo := device.NewRepository(txDB)
		otpRepo := authotp.NewRepository(txDB)
		authRepo := NewRespository(txDB)

		sessionTokenHash := sha256.Sum256([]byte(sessionToken))
		hashedSessionToken := hex.EncodeToString(sessionTokenHash[:])

		pendingSession, err := deviceRepo.GetPendingSessionByHash(ctx, hashedSessionToken)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return appErr.ErrInvalidSession
			}
			return err
		}

		now := time.Now().UTC()
		if pendingSession.IsUsed() || pendingSession.IsExpired(now) {
			return appErr.ErrInvalidSession
		}
		if strings.TrimSpace(pendingSession.DeviceID) != deviceID {
			return appErr.ErrInvalidSession
		}
		if strings.TrimSpace(pendingSession.OTPRef) == "" {
			return appErr.ErrInvalidSession
		}

		activeOTP, err := otpRepo.GetActiveOTPByID(ctx, strings.TrimSpace(pendingSession.OTPRef), loginOTPPurpose)
		if err != nil {
			return err
		}
		if activeOTP == nil {
			return appErr.ErrInvalidOTP
		}

		maxAttempts := activeOTP.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		if activeOTP.AttemptCount >= maxAttempts {
			return appErr.ErrInvalidOTP
		}

		hashedOTP, err := authotp.HashOTP(s.otpPepper, loginOTPPurpose, activeOTP.Destination, otp)
		if err != nil || !authotp.HashEqualHex(hashedOTP, activeOTP.OTPHash) {
			if updateErr := otpRepo.IncrementAttempt(ctx, activeOTP.ID); updateErr != nil {
				return updateErr
			}
			return appErr.ErrInvalidOTP
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
			return appErr.ErrInvalidSession
		}

		authObj, err = s.issueSessionTokensWithRepo(ctx, authRepo, pendingSession.UserID, deviceID, ip)
		if err != nil {
			return err
		}

		user, err := authRepo.GetUserByID(ctx, pendingSession.UserID)
		if err != nil {
			return err
		}

		authObj.IsBiometricsEnabled = user.IsBiometricsEnabled

		return nil
	})
	if err != nil {
		return nil, err
	}

	return authObj, nil
}

func (s *Service) startNewDeviceFlow(ctx context.Context, userID, phone, deviceID, ip string) (*LoginInitObject, error) {
	if s.tx == nil {
		return nil, errors.New("transaction manager not configured")
	}
	if s.smsSender == nil {
		return nil, errors.New("sms sender not configured")
	}
	if strings.TrimSpace(s.otpPepper) == "" {
		return nil, errors.New("otp pepper not configured")
	}

	normalizedPhone, err := phoneutil.NormalizeNigerianNumber(phone)
	if err != nil {
		return nil, err
	}

	otpResult, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     loginOTPPurpose,
		Channel:     loginOTPChannel,
		Destination: normalizedPhone,
		UserID:      userID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})
	if err != nil {
		return nil, err
	}

	var sessionToken string
	err = s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		deviceRepo := device.NewRepository(txDB)
		token, err := s.createPendingDeviceSession(ctx, deviceRepo, userID, deviceID, ip, otpResult.OTPID)
		if err != nil {
			return err
		}
		sessionToken = token

		authRepo := NewRespository(txDB)
		if err := authRepo.DeactiveOlderDevices(ctx, userID, deviceID); err != nil {
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

func (s *Service) createPendingDeviceSession(ctx context.Context, deviceRepo *device.Repository, userID, deviceID, ip, otpRef string) (string, error) {
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

func (s *Service) VerifyDeviceChallenge(ctx context.Context, challenge, signature, deviceID, ip string) (*VerifiedDeviceResponse, error) {
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
		return nil, appErr.ErrMissingDeviceID
	}

	if s.deviceRepo == nil {
		return nil, errors.New("device repository not configured")
	}

	challengeHash := sha256.Sum256([]byte(challenge))
	storedChallenge, err := s.deviceRepo.GetChallengeByHash(ctx, hex.EncodeToString(challengeHash[:]))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrInvalidSession
		}
		return nil, err
	}

	now := time.Now().UTC()
	if storedChallenge.IsUsed() || storedChallenge.IsExpired(now) {
		return nil, appErr.ErrInvalidSession
	}

	if storedChallenge.DeviceID != deviceID {
		return nil, appErr.ErrInvalidSession
	}

	deviceRecord, err := s.deviceVerifier.VerifyUserDevice(ctx, storedChallenge.UserID, storedChallenge.DeviceID)
	if err != nil {
		return nil, errors.New("device verification failed")
	}

	validSig, err := verifyDeviceSignature(deviceRecord.PublicKey, challenge, signature)
	if err != nil || !validSig {
		return nil, errors.New("device verification failed")
	}

	marked, err := s.deviceRepo.MarkChallengeUsed(ctx, storedChallenge.ID, now)
	if err != nil {
		return nil, err
	}
	if !marked {
		return nil, appErr.ErrInvalidSession
	}

	if err := s.deviceRepo.UpdateLastUsed(ctx, deviceRecord.UserID, deviceRecord.DeviceID, now); err != nil {
		return nil, err
	}

	resp, err := s.issueSessionTokens(ctx, storedChallenge.UserID, deviceRecord.DeviceID, ip)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, storedChallenge.UserID)
	if err == nil {
		resp.IsBiometricsEnabled = user.IsBiometricsEnabled
	}

	return resp, nil
}

func (s *Service) ResendNewDeviceOTP(ctx context.Context, req ResendNewDeviceOTPRequest) error {
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
		return appErr.ErrMissingDeviceID
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		deviceRepo := device.NewRepository(txDB)
		authRepo := NewRespository(txDB)

		sum := sha256.Sum256([]byte(req.SessionToken))
		session, err := deviceRepo.GetPendingSessionByHash(ctx, hex.EncodeToString(sum[:]))
		if err != nil {
			return appErr.ErrInvalidSession
		}

		now := time.Now().UTC()
		if session.IsUsed() || session.IsExpired(now) || strings.TrimSpace(session.DeviceID) != req.DeviceID {
			return appErr.ErrInvalidSession
		}

		user, err := authRepo.GetUserByID(ctx, session.UserID)
		if err != nil {
			return err
		}

		phone, err := phoneutil.NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return err
		}

		otpResult, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
			Purpose:     loginOTPPurpose,
			Channel:     loginOTPChannel,
			Destination: phone,
			UserID:      session.UserID,
			TTL:         10 * time.Minute,
			MaxAttempts: 5,
			MaxResends:  3,
		})
		if err != nil {
			return err
		}

		return deviceRepo.RefreshPendingSession(ctx, session.ID, otpResult.OTPID, now.Add(10*time.Minute), now)
	})
}

func (s *Service) ToggleBiometrics(ctx context.Context, mobileUserID, deviceID string) (*ToggleBiometricsResponse, error) {
	if strings.TrimSpace(mobileUserID) == "" {
		return nil, errors.New("mobile user id is required")
	}

	if strings.TrimSpace(deviceID) == "" {
		return nil, appErr.ErrMissingDeviceID
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	enabled, err := s.repo.ToggleBiometrics(ctx, mobileUserID)
	if err != nil {
		return nil, errors.New("unable to toggle biometrics")
	}

	var message string

	switch enabled {
	case true:
		message = "biometrics has been enabled"
	case false:
		message = "biometrics has been disabled"
	}

	return &ToggleBiometricsResponse{
		Status:    "success",
		Message:   message,
		IsEnabled: enabled,
	}, nil
}

func (s *Service) issueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*VerifiedDeviceResponse, error) {
	return s.issueSessionTokensWithRepo(ctx, s.repo, userID, deviceID, ip)
}

func (s *Service) issueSessionTokensWithRepo(ctx context.Context, repo *Repository, userID, deviceID, ip string) (*VerifiedDeviceResponse, error) {
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

	return &VerifiedDeviceResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
