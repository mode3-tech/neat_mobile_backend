package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/helpers"
	"neat_mobile_app_backend/internal/middleware"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"
	authotp "neat_mobile_app_backend/internal/modules/auth/otp"
	"neat_mobile_app_backend/internal/modules/device"
	phoneutil "neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// logLoginAudit records a login attempt (the phone/password phase). extra carries any
// additional metadata to merge in; err, when non-nil, marks the attempt as a failure and
// its message is recorded under "reason_for_failure".
func (s *Service) logLoginAudit(ctx context.Context, phone string, extra map[string]interface{}, err error) {
	status := auditlog.StatusSuccess
	metadata := extra
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if err != nil {
		status = auditlog.StatusFailure
		metadata["reason_for_failure"] = err.Error()
	}
	s.auditLogger.CreateAuditLog(ctx, &auditlog.AuditLog{
		ID:           helpers.PrefixID("audit_log"),
		ResourceID:   MaskSub(phone),
		ResourceType: auditlog.ResourceTypeLogin,
		ActorType:    "unauthenticated",
		RequestID:    middleware.GetRequestID(ctx),
		Timestamp:    time.Now().UTC(),
		Status:       status,
		ActorID:      MaskSub(phone),
		Action:       "LOGIN",
		IPAddress:    middleware.GetClientIP(ctx),
		Metadata:     metadata,
	})
}

// logLoginSuccess records that a user actually completed login (i.e. session tokens were
// issued), as distinct from the intermediate device/challenge steps that got them there.
// Call this once, right after a login-completing token issuance succeeds.
func (s *Service) logLoginSuccess(ctx context.Context, userID, phone, deviceID string) {
	s.auditLogger.CreateAuditLog(ctx, &auditlog.AuditLog{
		ID:           helpers.PrefixID("audit_log"),
		ResourceID:   MaskSub(phone),
		ResourceType: auditlog.ResourceTypeLogin,
		ActorType:    "user",
		RequestID:    middleware.GetRequestID(ctx),
		Timestamp:    time.Now().UTC(),
		Status:       auditlog.StatusSuccess,
		ActorID:      userID,
		Action:       "LOGIN",
		IPAddress:    middleware.GetClientIP(ctx),
		Metadata:     map[string]interface{}{"device_id": deviceID},
	})
}

// logAuthEvent is the general-purpose audit helper for every other step in the login /
// device-verification lifecycle (challenge creation, new-device flow, OTP resend, session
// issuance, biometrics toggle). err, when non-nil, marks the event as a failure and its
// message is recorded under "reason_for_failure"; actorID left empty means the actor
// hasn't been identified yet (e.g. before a device/user lookup succeeds).
func (s *Service) logAuthEvent(ctx context.Context, resourceType auditlog.ResourceType, action, actorID, resourceID string, err error, extra map[string]interface{}) {
	status := auditlog.StatusSuccess
	metadata := extra
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if err != nil {
		status = auditlog.StatusFailure
		metadata["reason_for_failure"] = err.Error()
	}
	actorType := "user"
	if actorID == "" {
		actorType = "unauthenticated"
	}
	s.auditLogger.CreateAuditLog(ctx, &auditlog.AuditLog{
		ID:           helpers.PrefixID("audit_log"),
		ResourceID:   resourceID,
		ResourceType: resourceType,
		ActorType:    actorType,
		RequestID:    middleware.GetRequestID(ctx),
		Timestamp:    time.Now().UTC(),
		Status:       status,
		ActorID:      actorID,
		Action:       action,
		IPAddress:    middleware.GetClientIP(ctx),
		Metadata:     metadata,
	})
}

func (s *Service) Login(ctx context.Context, deviceID, ip, phone, password string) (*LoginInitObject, error) {
	normalizedPhone, err := phoneutil.NormalizeNigerianNumber(phone)
	if err != nil {
		s.logLoginAudit(ctx, phone, nil, err)
		return nil, err
	}

	user, err := s.repo.GetUserByPhone(ctx, normalizedPhone)
	if err != nil {
		s.logLoginAudit(ctx, normalizedPhone, map[string]interface{}{"reason": "user not found"}, appErr.ErrInvalidCredentials)
		return nil, appErr.ErrInvalidCredentials
	}

	if user.ClosedAt != nil {
		s.logLoginAudit(ctx, normalizedPhone, map[string]interface{}{"user_id": user.ID, "reason": "account closed"}, appErr.ErrInvalidCredentials)
		return nil, appErr.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)

	if err != nil {
		s.logLoginAudit(ctx, normalizedPhone, map[string]interface{}{"user_id": user.ID, "reason": "invalid password"}, appErr.ErrInvalidCredentials)
		return nil, appErr.ErrInvalidCredentials
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		s.logLoginAudit(ctx, normalizedPhone, map[string]interface{}{"user_id": user.ID}, appErr.ErrMissingDeviceID)
		return nil, appErr.ErrMissingDeviceID
	}

	if s.deviceRepo == nil {
		log.Println("device repository not configured")
		cfgErr := errors.New("device repository not configured")
		s.logLoginAudit(ctx, normalizedPhone, map[string]interface{}{"user_id": user.ID}, cfgErr)
		return nil, cfgErr
	}

	deviceRecord, err := s.deviceRepo.FindDevice(ctx, user.ID, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logLoginAudit(ctx, normalizedPhone, map[string]interface{}{"user_id": user.ID, "device_id": deviceID, "reason": "new device detected"}, nil)
			return s.startNewDeviceFlow(ctx, user.ID, user.Phone, deviceID, ip)
		}
		s.logLoginAudit(ctx, normalizedPhone, map[string]interface{}{"user_id": user.ID, "device_id": deviceID}, err)
		return nil, err
	}

	if !deviceRecord.IsActive || !deviceRecord.IsTrusted {
		s.logLoginAudit(ctx, normalizedPhone, map[string]interface{}{"user_id": user.ID, "device_id": deviceID, "reason": "device inactive or untrusted"}, nil)
		return s.startNewDeviceFlow(ctx, user.ID, user.Phone, deviceID, ip)
	}

	deviceService := device.NewService(*s.deviceRepo, s.auditLogger)
	challenge, err := deviceService.CreateChallenge(ctx, user.ID, deviceID, 0)
	if err != nil {
		// deviceService.CreateChallenge already records its own DEVICE_CHALLENGE_CREATE
		// failure audit entry; avoid duplicating it here.
		log.Println("Error creating device challenge:", err)
		return nil, err
	}

	log.Printf("Device challenge created for user %s on device %s and challenge = %s", user.ID, deviceID, challenge)

	s.logLoginAudit(ctx, normalizedPhone, map[string]interface{}{"user_id": user.ID, "device_id": deviceID, "stage": "challenge_issued"}, nil)


	return &LoginInitObject{
		Status:    LoginStatusChallengeRequired,
		Challenge: challenge,
	}, nil
}

// CreateChallenge creates a nonce for biometric authentication. The device ID only
// identifies the registered key; authentication completes when its signature is
// verified by VerifyDeviceChallenge.
func (s *Service) CreateChallenge(ctx context.Context, deviceID string) (*ChallengeRequestResponse, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "CREATE_CHALLENGE", "", "", appErr.ErrMissingDeviceID, nil)
		return nil, appErr.ErrMissingDeviceID
	}
	if s.deviceRepo == nil {
		cfgErr := errors.New("device repository not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "CREATE_CHALLENGE", "", deviceID, cfgErr, nil)
		return nil, cfgErr
	}

	deviceRecord, err := s.deviceRepo.FindDeviceByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "CREATE_CHALLENGE", "", deviceID, appErr.ErrInvalidSession, map[string]interface{}{"reason": "device not found"})
			return nil, appErr.ErrInvalidSession
		}
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "CREATE_CHALLENGE", "", deviceID, err, nil)
		return nil, err
	}
	if !deviceRecord.IsActive || !deviceRecord.IsTrusted || strings.TrimSpace(deviceRecord.PublicKey) == "" {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "CREATE_CHALLENGE", deviceRecord.UserID, deviceID, appErr.ErrInvalidSession, map[string]interface{}{"reason": "device inactive, untrusted, or missing public key"})
		return nil, appErr.ErrInvalidSession
	}

	const ttl = 60 * time.Second
	deviceService := device.NewService(*s.deviceRepo, s.auditLogger)
	challenge, err := deviceService.CreateChallenge(ctx, deviceRecord.UserID, deviceRecord.DeviceID, ttl)
	if err != nil {
		// deviceService.CreateChallenge already records its own DEVICE_CHALLENGE_CREATE
		// failure audit entry; avoid duplicating it here.
		return nil, err
	}

	s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "CREATE_CHALLENGE", deviceRecord.UserID, deviceID, nil, nil)

	return &ChallengeRequestResponse{
		Status:    LoginStatusChallengeRequired,
		Message:   "challenge created successfully",
		Challenge: challenge,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}, nil
}

func (s *Service) VerifyNewDevice(ctx context.Context, ip string, req NewDeviceResquest) (*VerifiedDeviceResponse, error) {
	deviceID := strings.TrimSpace(req.Device.DeviceID)

	if s.tx == nil {
		cfgErr := errors.New("transaction manager not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_NEW_DEVICE", "", deviceID, cfgErr, nil)
		return nil, cfgErr
	}
	if s.deviceRepo == nil {
		cfgErr := errors.New("device repository not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_NEW_DEVICE", "", deviceID, cfgErr, nil)
		return nil, cfgErr
	}
	if strings.TrimSpace(s.otpPepper) == "" {
		cfgErr := errors.New("otp pepper not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_NEW_DEVICE", "", deviceID, cfgErr, nil)
		return nil, cfgErr
	}

	sessionToken := strings.TrimSpace(req.SessionToken)
	otp := strings.TrimSpace(req.OTP)

	var authObj *VerifiedDeviceResponse
	var verifiedUserID string
	var verifiedPhone string

	err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		deviceRepo := device.NewRepository(txDB)
		otpRepo := authotp.NewRepository(txDB)
		authRepo := NewRespository(txDB, s.repo.cipher)

		sessionTokenHash := sha256.Sum256([]byte(sessionToken))
		hashedSessionToken := hex.EncodeToString(sessionTokenHash[:])

		pendingSession, err := deviceRepo.GetPendingSessionByHash(ctx, hashedSessionToken)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return appErr.ErrInvalidSession
			}
			return err
		}
		verifiedUserID = pendingSession.UserID

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
			ID:          uuid.NewString(),
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
			log.Printf("auth service: failed to upsert device public key - %s\n", err)
			return err
		}
		if err := deviceRepo.ActivateAndTrustDevice(ctx, pendingSession.UserID, deviceID, now, ip); err != nil {
			log.Printf("auth service: failed to activate device - %s\n", err)
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
		verifiedPhone = user.Phone

		authObj.IsBiometricsEnabled = user.IsBiometricsEnabled

		return nil
	})
	if err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_NEW_DEVICE", verifiedUserID, deviceID, err, nil)
		return nil, err
	}

	s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_NEW_DEVICE", verifiedUserID, deviceID, nil, nil)
	s.logLoginSuccess(ctx, verifiedUserID, verifiedPhone, deviceID)

	return authObj, nil
}

func (s *Service) startNewDeviceFlow(ctx context.Context, userID, phone, deviceID, ip string) (*LoginInitObject, error) {
	if s.tx == nil {
		cfgErr := errors.New("transaction manager not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "NEW_DEVICE_FLOW_STARTED", userID, deviceID, cfgErr, nil)
		return nil, cfgErr
	}
	if s.smsSender == nil {
		cfgErr := errors.New("sms sender not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "NEW_DEVICE_FLOW_STARTED", userID, deviceID, cfgErr, nil)
		return nil, cfgErr
	}
	if strings.TrimSpace(s.otpPepper) == "" {
		cfgErr := errors.New("otp pepper not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "NEW_DEVICE_FLOW_STARTED", userID, deviceID, cfgErr, nil)
		return nil, cfgErr
	}

	normalizedPhone, err := phoneutil.NormalizeNigerianNumber(phone)
	if err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "NEW_DEVICE_FLOW_STARTED", userID, deviceID, err, nil)
		return nil, err
	}

	otpResult, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     loginOTPPurpose,
		Channel:     loginOTPChannel,
		Destination: normalizedPhone,
		UserID:      userID,
		TTL:         authotp.DefaultOTPSMSTTL,
		MaxAttempts: 5,
		MaxResends:  3,
	})
	if err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "NEW_DEVICE_FLOW_STARTED", userID, deviceID, err, map[string]interface{}{"reason": "failed to issue otp"})
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

		authRepo := NewRespository(txDB, s.repo.cipher)
		if err := authRepo.DeactiveOlderDevices(ctx, userID, deviceID); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "NEW_DEVICE_FLOW_STARTED", userID, deviceID, err, nil)
		return nil, err
	}

	s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "NEW_DEVICE_FLOW_STARTED", userID, deviceID, nil, nil)

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
		err := errors.New("challenge is required")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", "", deviceID, err, nil)
		return nil, err
	}

	signature = strings.TrimSpace(signature)
	if signature == "" {
		err := errors.New("signature is required")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", "", deviceID, err, nil)
		return nil, err
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", "", deviceID, appErr.ErrMissingDeviceID, nil)
		return nil, appErr.ErrMissingDeviceID
	}

	if s.deviceRepo == nil {
		cfgErr := errors.New("device repository not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", "", deviceID, cfgErr, nil)
		return nil, cfgErr
	}

	challengeHash := sha256.Sum256([]byte(challenge))
	storedChallenge, err := s.deviceRepo.GetChallengeByHash(ctx, hex.EncodeToString(challengeHash[:]))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", "", deviceID, appErr.ErrInvalidSession, map[string]interface{}{"reason": "challenge not found"})
			return nil, appErr.ErrInvalidSession
		}
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", "", deviceID, err, nil)
		return nil, err
	}

	now := time.Now().UTC()
	if storedChallenge.IsUsed() || storedChallenge.IsExpired(now) {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, appErr.ErrInvalidSession, map[string]interface{}{"reason": "challenge used or expired"})
		return nil, appErr.ErrInvalidSession
	}

	if storedChallenge.DeviceID != deviceID {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, appErr.ErrInvalidSession, map[string]interface{}{"reason": "device id mismatch"})
		return nil, appErr.ErrInvalidSession
	}

	deviceRecord, err := s.deviceVerifier.VerifyUserDevice(ctx, storedChallenge.UserID, storedChallenge.DeviceID)
	if err != nil {
		if errors.Is(err, appErr.ErrUnrecognizedDevice) {
			s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, appErr.ErrUnrecognizedDeviceBiometricLogin, nil)
			return nil, appErr.ErrUnrecognizedDeviceBiometricLogin
		}
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, err, nil)
		return nil, err
	}

	validSig, err := verifyDeviceSignature(deviceRecord.PublicKey, challenge, signature)
	if err != nil || !validSig {
		sigErr := err
		if sigErr == nil {
			sigErr = errors.New("signature verification failed")
		}
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, appErr.ErrInvalidSession, map[string]interface{}{"reason": sigErr.Error()})
		return nil, appErr.ErrInvalidSession
	}

	marked, err := s.deviceRepo.MarkChallengeUsed(ctx, storedChallenge.ID, now)
	if err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, err, nil)
		return nil, err
	}
	if !marked {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, appErr.ErrInvalidSession, map[string]interface{}{"reason": "challenge already used"})
		return nil, appErr.ErrInvalidSession
	}

	if err := s.deviceRepo.UpdateLastUsed(ctx, deviceRecord.UserID, deviceRecord.DeviceID, now); err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, err, nil)
		return nil, err
	}

	resp, err := s.issueSessionTokens(ctx, storedChallenge.UserID, deviceRecord.DeviceID, ip)
	if err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, err, nil)
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, storedChallenge.UserID)
	var phone string
	if err == nil {
		resp.IsBiometricsEnabled = user.IsBiometricsEnabled
		phone = user.Phone
	}

	s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "VERIFY_DEVICE_CHALLENGE", storedChallenge.UserID, deviceID, nil, nil)
	s.logLoginSuccess(ctx, storedChallenge.UserID, phone, deviceID)

	return resp, nil
}

func (s *Service) ResendNewDeviceOTP(ctx context.Context, req ResendNewDeviceOTPRequest) error {
	if s.tx == nil {
		cfgErr := errors.New("transaction manager not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "RESEND_NEW_DEVICE_OTP", "", req.DeviceID, cfgErr, nil)
		return cfgErr
	}
	if s.smsSender == nil {
		cfgErr := errors.New("sms sender not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "RESEND_NEW_DEVICE_OTP", "", req.DeviceID, cfgErr, nil)
		return cfgErr
	}
	if strings.TrimSpace(s.otpPepper) == "" {
		cfgErr := errors.New("otp pepper not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "RESEND_NEW_DEVICE_OTP", "", req.DeviceID, cfgErr, nil)
		return cfgErr
	}

	req.SessionToken = strings.TrimSpace(req.SessionToken)
	if req.SessionToken == "" {
		err := errors.New("session token is required")
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "RESEND_NEW_DEVICE_OTP", "", req.DeviceID, err, nil)
		return err
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "RESEND_NEW_DEVICE_OTP", "", req.DeviceID, appErr.ErrMissingDeviceID, nil)
		return appErr.ErrMissingDeviceID
	}

	var resendUserID string

	err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		deviceRepo := device.NewRepository(txDB)
		authRepo := NewRespository(txDB, s.repo.cipher)

		sum := sha256.Sum256([]byte(req.SessionToken))
		session, err := deviceRepo.GetPendingSessionByHash(ctx, hex.EncodeToString(sum[:]))
		if err != nil {
			return appErr.ErrInvalidSession
		}
		resendUserID = session.UserID

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
			TTL:         authotp.DefaultOTPSMSTTL,
			MaxAttempts: 5,
			MaxResends:  3,
		})
		if err != nil {
			return err
		}

		return deviceRepo.RefreshPendingSession(ctx, session.ID, otpResult.OTPID, now.Add(10*time.Minute), now)
	})
	if err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "RESEND_NEW_DEVICE_OTP", resendUserID, req.DeviceID, err, nil)
		return err
	}

	s.logAuthEvent(ctx, auditlog.ResourceTypeDevice, "RESEND_NEW_DEVICE_OTP", resendUserID, req.DeviceID, nil, nil)

	return nil
}

func (s *Service) ToggleBiometrics(ctx context.Context, mobileUserID string) (*ToggleBiometricsResponse, error) {
	enabled, err := s.repo.ToggleBiometrics(ctx, mobileUserID)
	if err != nil {
		toggleErr := errors.New("unable to toggle biometrics")
		s.logAuthEvent(ctx, auditlog.ResourceTypeUser, "TOGGLE_BIOMETRICS", mobileUserID, mobileUserID, toggleErr, map[string]interface{}{"reason": err.Error()})
		return nil, toggleErr
	}

	s.logAuthEvent(ctx, auditlog.ResourceTypeUser, "TOGGLE_BIOMETRICS", mobileUserID, mobileUserID, nil, map[string]interface{}{"is_enabled": enabled})

	return &ToggleBiometricsResponse{
		IsEnabled: enabled,
	}, nil
}

func (s *Service) issueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*VerifiedDeviceResponse, error) {
	return s.issueSessionTokensWithRepo(ctx, s.repo, userID, deviceID, ip)
}

// IssueSessionTokens is the exported entry point for other registration flows
// (e.g. registerv2) that need to issue a session the same way the primary
// login/registration flow does.
func (s *Service) IssueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*VerifiedDeviceResponse, error) {
	return s.issueSessionTokensWithRepo(ctx, s.repo, userID, deviceID, ip)
}

func (s *Service) issueSessionTokensWithRepo(ctx context.Context, repo *Repository, userID, deviceID, ip string) (*VerifiedDeviceResponse, error) {
	if repo == nil {
		cfgErr := errors.New("auth repository not configured")
		s.logAuthEvent(ctx, auditlog.ResourceTypeSession, "ISSUE_SESSION_TOKENS", userID, deviceID, cfgErr, nil)
		return nil, cfgErr
	}

	sid := uuid.NewString()

	accessToken, err := s.jwtSigner.IssueAccessToken(userID, sid)
	if err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeSession, "ISSUE_SESSION_TOKENS", userID, sid, err, map[string]interface{}{"reason": "failed to issue access token"})
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
		s.logAuthEvent(ctx, auditlog.ResourceTypeSession, "ISSUE_SESSION_TOKENS", userID, sid, err, map[string]interface{}{"reason": "failed to persist access token session"})
		return nil, err
	}

	refreshToken, jti, refreshExpiresAt, err := s.jwtSigner.IssueRefreshToken(userID, sid)
	if err != nil {
		s.logAuthEvent(ctx, auditlog.ResourceTypeSession, "ISSUE_SESSION_TOKENS", userID, sid, err, map[string]interface{}{"reason": "failed to issue refresh token"})
		return nil, err
	}

	hashedRefreshToken := sha256.Sum256([]byte(refreshToken))
	if hashedRefreshToken == [32]byte{} {
		hashErr := errors.New("error while hashing refresh token")
		s.logAuthEvent(ctx, auditlog.ResourceTypeSession, "ISSUE_SESSION_TOKENS", userID, sid, hashErr, nil)
		return nil, hashErr
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
		s.logAuthEvent(ctx, auditlog.ResourceTypeSession, "ISSUE_SESSION_TOKENS", userID, sid, err, map[string]interface{}{"reason": "failed to persist refresh token"})
		return nil, err
	}

	s.logAuthEvent(ctx, auditlog.ResourceTypeSession, "ISSUE_SESSION_TOKENS", userID, sid, nil, map[string]interface{}{"device_id": deviceID})

	return &VerifiedDeviceResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
