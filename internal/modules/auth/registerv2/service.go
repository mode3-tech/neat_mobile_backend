package registerv2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/authchecker"
	"neat_mobile_app_backend/internal/database/tx"
	appErr "neat_mobile_app_backend/internal/errors"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"
	"neat_mobile_app_backend/internal/modules/auth"
	"neat_mobile_app_backend/internal/modules/auth/otp"
	"neat_mobile_app_backend/internal/modules/device"
	phoneutil "neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/models"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const verificationExpiry = 30 * time.Minute

const (
	// bvnValidateMaxAttempts is the initial attempt plus two retries.
	bvnValidateMaxAttempts = 3
	bvnValidateRetryDelay  = 750 * time.Millisecond
	// bvnValidateOverallBudget caps the whole retry loop, including the
	// Optimus client's own 15s-per-call timeout (providers/baas/optimus.go) -
	// three naive full-length attempts could otherwise exceed the server's
	// 30s WriteTimeout (internal/server/server.go) and get the response cut
	// off mid-flight. Kept comfortably under that.
	bvnValidateOverallBudget = 25 * time.Second
)

type Service struct {
	repo                    *Repository
	providerPreference      ProviderPreferenceRepository
	optimus                 OptimusValidator
	walletGenerator         auth.WalletService
	providusWalletGenerator auth.WalletService
	sessionIssuer           SessionIssuer
	otpManager              otp.OTPManager
	tx                      *tx.Transactor
	activationCapKobo       int64
	optimusProductID        string
	auditLogger             auditlog.AuditLogger
	alat                    AlatValidator
	// demoPhone is the normalized whitelisted demo phone (DEMO_LOGIN_PHONE).
	// Registration for it skips wallet generation and user inserts; empty disables it.
	demoPhone string

	// Test seams; zero values mean production behaviour.
	wemaPollInterval time.Duration // between ALAT account polls (default 1s)
	lockOverride     func(ctx context.Context, keys ...string) (release func(), ok bool, err error)
	persistOverride  func(ctx context.Context, rl *opLogger, reg *registration, walletResp *auth.WalletResponse, provider string, inTx func(*gorm.DB) error) error
}

func (s *Service) SetAlatValidator(alat AlatValidator) {
	s.alat = alat
}

func NewService(
	repo *Repository,
	providerPreference ProviderPreferenceRepository,
	optimus OptimusValidator,
	walletGenerator auth.WalletService,
	providusWalletGenerator auth.WalletService,
	sessionIssuer SessionIssuer,
	otpManager otp.OTPManager,
	transactor *tx.Transactor,
	activationCapKobo int64,
	optimusProductID string,
	auditLogger auditlog.AuditLogger,
	demoPhone string,
) *Service {
	return &Service{
		repo:                    repo,
		providerPreference:      providerPreference,
		optimus:                 optimus,
		walletGenerator:         walletGenerator,
		providusWalletGenerator: providusWalletGenerator,
		sessionIssuer:           sessionIssuer,
		otpManager:              otpManager,
		tx:                      transactor,
		activationCapKobo:       activationCapKobo,
		optimusProductID:        optimusProductID,
		auditLogger:             auditLogger,
		demoPhone:               strings.TrimSpace(demoPhone),
	}
}

func (s *Service) lockRegistration(ctx context.Context, keys ...string) (func(), bool, error) {
	if s.lockOverride != nil {
		return s.lockOverride(ctx, keys...)
	}
	return s.repo.LockRegistration(ctx, keys...)
}

// CurrentProvider returns which BaaS provider should be used for the next
// registration.
func (s *Service) CurrentProvider(ctx context.Context) (string, error) {
	pref, err := s.providerPreference.GetProviderPreference(ctx)
	if err != nil {
		return "", err
	}
	return pref.Provider, nil
}

// Register validates the four verification records (phone/email by ID,
// BVN/NIN by re-hashing the raw value), provisions a wallet via the current
// provider, creates the user, and issues a session. Kept synchronous -
// Optimus's GenerateWallet is a single blocking call, unlike the old flow's
// async job/poll/claim pattern.
func (s *Service) Register(ctx context.Context, req OptimusRegisterRequest, ip string) (_ *RegisterResponse, retErr error) {
	rl := newOpLogger("register")
	defer rl.finish(&retErr)

	rl.step("input")
	if err := authchecker.ValidatePassword(req.Password); err != nil {
		return nil, asUserError(err)
	}
	if req.Password != req.ConfirmPassword {
		return nil, userErrorf("password and confirm password do not match")
	}
	if err := authchecker.ValidatePin(req.TransactionPin); err != nil {
		return nil, asUserError(err)
	}
	if req.TransactionPin != req.ConfirmTransactionPin {
		return nil, userErrorf("transaction pin and confirm transaction pin do not match")
	}

	normalizedPhone, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(req.PhoneNumber))
	if err != nil {
		rl.with("phone", maskPhone(req.PhoneNumber))
		return nil, err
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))
	rl.with("phone", maskPhone(normalizedPhone)).
		with("email", maskEmail(normalizedEmail)).
		with("bvn", maskIdentityNumber(strings.TrimSpace(req.BVN)))

	rl.step("phone_verification")
	phoneRecord, err := s.repo.GetVerificationByID(ctx, strings.TrimSpace(req.PhoneVerificationID))
	if err != nil {
		return nil, err
	}
	if phoneRecord == nil || phoneRecord.Status != models.VerificationStatusVerified ||
		phoneRecord.Type != models.VerificationTypePhone ||
		phoneRecord.VerifiedPhone == nil || strings.TrimSpace(*phoneRecord.VerifiedPhone) != normalizedPhone {
		rl.Warnf("phone verification rejected: %s", describeContactRecord(phoneRecord, models.VerificationTypePhone,
			phoneRecord != nil && phoneRecord.VerifiedPhone != nil && strings.TrimSpace(*phoneRecord.VerifiedPhone) == normalizedPhone))
		return nil, userErrorf("phone number is not verified")
	}

	rl.step("email_verification")
	emailRecord, err := s.repo.GetVerificationByID(ctx, strings.TrimSpace(req.EmailVerificationID))
	if err != nil {
		return nil, err
	}
	if emailRecord == nil || emailRecord.Status != models.VerificationStatusVerified ||
		emailRecord.Type != models.VerificationTypeEmail ||
		emailRecord.VerifiedEmail == nil || strings.ToLower(strings.TrimSpace(*emailRecord.VerifiedEmail)) != normalizedEmail {
		rl.Warnf("email verification rejected: %s", describeContactRecord(emailRecord, models.VerificationTypeEmail,
			emailRecord != nil && emailRecord.VerifiedEmail != nil && strings.ToLower(strings.TrimSpace(*emailRecord.VerifiedEmail)) == normalizedEmail))
		return nil, userErrorf("email is not verified")
	}

	// NIN is collected and persisted below but no longer gates registration -
	// it isn't sent to either provider's wallet-creation call, so validating
	// it up front only added friction without changing the outcome.

	// One registration per person at a time: without this, two parallel
	// calls both pass the duplicate checks below, both reach the provider,
	// and only one can be saved - the other's account is orphaned. Held
	// until Register returns.
	rl.step("registration_lock")
	release, locked, err := s.lockRegistration(ctx, "phone:"+normalizedPhone, "bvn:"+bvnSubjectHash(req.BVN))
	if err != nil {
		return nil, err
	}
	if !locked {
		rl.Warnf("another registration for this phone or bvn is in progress")
		s.logRejectedTier2Payload(ctx, rl, req, normalizedPhone, normalizedEmail)
		return nil, userErrorf("a registration for this phone number or BVN is already in progress, please wait a moment and try again")
	}
	defer release()

	// The demo account is the exception: it already exists, so reuse it rather
	// than provisioning another wallet or inserting a second user.
	rl.step("duplicate_check")
	authRepo := auth.NewRespository(s.repo.db, s.repo.cipher)
	var existingUser *models.User
	isDemo := false
	if u, lookupErr := authRepo.GetUserByPhone(ctx, normalizedPhone); lookupErr == nil {
		if s.demoPhone == "" || normalizedPhone != s.demoPhone {
			return nil, appErr.ErrUserExists
		}
		existingUser = u
		isDemo = true
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return nil, lookupErr
	}
	if isDemo {
		return s.registerDemo(ctx, existingUser, req, ip)
	}

	// Same reasoning for BVN/NIN: wallet_users.bvn_hash/nin_hash are unique,
	// so a duplicate would otherwise only fail at CreateUser - after the
	// provider account had already been created and left orphaned.
	if exists, existsErr := s.repo.UserExistsByIdentity(ctx, req.BVN, req.NIN); existsErr != nil {
		return nil, existsErr
	} else if exists {
		rl.Warnf("user already exists with this bvn or nin (nin=%s)", maskIdentityNumber(strings.TrimSpace(req.NIN)))
		return nil, appErr.ErrUserExists
	}

	rl.step("provider_preference")
	provider, err := s.CurrentProvider(ctx)
	if err != nil {
		return nil, err
	}
	rl.with("provider", strings.TrimSpace(provider))

	var latestBVNRecord *models.VerificationRecord
	// requestedFace is the face check named by bvn_w_face_verification_id, if
	// any - loaded once here and reused by the wema branch below.
	var requestedFace *models.FaceCheckRecord
	if strings.TrimSpace(provider) != "providus" {
		rl.step("bvn_record")
		rec, face, err := s.resolveBVNRecord(ctx, req.BVN, req.BVNWithFaceVerificationID)
		if err != nil {
			return nil, err
		}
		latestBVNRecord = rec
		requestedFace = face
		rl.Infof("bvn record resolved: %s; requested_face=%s", describeBVNRecord(rec), describeFaceRecord(face))
	}
	if err := checkBVNRecordUsable(latestBVNRecord); err != nil {
		return nil, err
	}

	rl.step("provider_select")

	// bvnRecord is only set (and later marked used) when Optimus/ALAT actually
	// verified this BVN during onboarding - nil whenever Providus is the
	// primary provider or Optimus's validation/OTP never completed.
	var bvnRecord *models.VerificationRecord
	var walletGenerator auth.WalletService
	// actualProvider tracks which provider's wallet-creation call is actually
	// used, as distinct from the preference in `provider` - the "optimus"
	// preference can still fall through to Providus below, and the wallet
	// record needs to know which provider genuinely issued the account (NUBANs
	// aren't portable between providers, so transfers need to route correctly).
	var actualProvider string

	switch strings.TrimSpace(provider) {
	case "providus":
		// Providus validates the BVN itself as part of wallet creation, so
		// there's nothing to look up here and no fallback needed on this path.
		walletGenerator = s.providusWalletGenerator
		actualProvider = "providus"

	case "optimus":
		if s.optimus == nil {
			rl.Errorf("optimus client is nil - check optimus wiring in router.go")
			return nil, fmt.Errorf("optimus service is not configured")
		}

		switch {
		case latestBVNRecord == nil:
			// /auth/validate/bvn was never called for this BVN - fallback only
			// applies to calls that were made and Optimus rejected, so this is
			// a hard stop, not a fallback trigger.
			return nil, userErrorf("bvn is not verified")

		case latestBVNRecord.Status == models.VerificationStatusVerified:
			// Optimus validated the BVN and its OTP challenge was confirmed - use it.
			bvnRecord = latestBVNRecord
			walletGenerator = s.walletGenerator
			actualProvider = "optimus"

		case latestBVNRecord.Status == models.VerificationStatusFailed:
			// Optimus's validate-bvn call itself rejected this BVN - fall back
			// to Providus, which performs its own check as part of wallet
			// creation.
			walletGenerator = s.providusWalletGenerator
			actualProvider = "providus"
			rl.Warnf("optimus rejected this bvn earlier (%s) - falling back to providus", describeBVNRecord(latestBVNRecord))

		default:
			// Pending: validate-bvn succeeded but the OTP challenge was never
			// confirmed (or failed). This is not a fallback case - falling back
			// here would let someone route around the OTP consent step just by
			// not completing it, so the user must resolve it instead.
			return nil, userErrorf("bvn otp verification is not complete")
		}

	case "wema", "alat":
		if s.alat == nil {
			rl.Errorf("alat client is nil - SetAlatValidator was not called")
			return nil, fmt.Errorf("wema alat service is not configured")
		}

		switch {
		case latestBVNRecord == nil:
			return nil, userErrorf("bvn is not verified")
		case latestBVNRecord.Status == models.VerificationStatusVerified:
			// Expired/used records were already rejected by checkBVNRecordUsable.
			bvnRecord = latestBVNRecord
			actualProvider = "wema"
		case latestBVNRecord.Status == models.VerificationStatusFailed:
			return nil, userErrorf("bvn verification failed")
		default:
			return nil, userErrorf("bvn otp verification is not complete")
		}

	default:
		return nil, fmt.Errorf("registration provider %q is not supported by this flow", provider)
	}
	rl.with("actual_provider", actualProvider)

	rl.step("input")

	dob, err := time.Parse("2006-01-02", strings.TrimSpace(req.Dob))
	if err != nil {
		return nil, &userError{msg: "invalid date of birth, use the format YYYY-MM-DD", err: err}
	}

	// The profile's name and DOB must be the BVN's, checked before anything is
	// created at the provider. Records from providers that don't return them
	// (e.g. Optimus) skip the check.
	rl.step("bvn_identity")
	if err := checkApplicantMatchesBVN(bvnRecord, req.FirstName, req.LastName, req.Dob); err != nil {
		rl.Warnf("applicant does not match bvn record %s: %v", bvnRecordID(bvnRecord), err)
		return nil, err
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	pinHash, err := auth.HashPassword(req.TransactionPin)
	if err != nil {
		return nil, err
	}

	reg := &registration{
		mobileUserID:      uuid.NewString(),
		internalWalletID:  uuid.NewString(),
		phone:             normalizedPhone,
		email:             normalizedEmail,
		firstName:         req.FirstName,
		lastName:          req.LastName,
		dobRaw:            strings.TrimSpace(req.Dob),
		dob:               dob,
		address:           strings.TrimSpace(req.Address),
		bvn:               strings.TrimSpace(req.BVN),
		nin:               strings.TrimSpace(req.NIN),
		passwordHash:      passwordHash,
		pinHash:           pinHash,
		biometrics:        *req.IsBiomtricsEnabled,
		referralCode:      strings.TrimSpace(req.ReferralCode),
		device:            req.Device,
		ip:                ip,
		phoneVerification: phoneRecord.ID,
		emailVerification: emailRecord.ID,
	}
	if bvnRecord != nil {
		reg.bvnVerification = bvnRecord.ID
	}
	rl.with("user_id", reg.mobileUserID)

	// We're about to trigger an external, irreversible side effect (wallet
	// creation at the provider) - from here on, work must not be abortable
	// just because the client disconnects (c.Request.Context() is canceled
	// when that happens). A canceled ctx during this window previously left
	// the provider account orphaned with nothing saved locally: the
	// wallet-generation HTTP call itself can time out client-side while the
	// provider keeps processing and completes the account anyway, and any
	// subsequent work on the original ctx would fail immediately once the
	// client gave up. providerCtx keeps request-scoped values but drops the
	// parent's cancellation, giving the provider call, the DB write, and
	// session issuance a guaranteed bounded runway of their own instead. 2
	// minutes comfortably covers GenerateWallet's own 90s timeout plus the DB
	// transaction and session issuance without being unbounded.
	providerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()

	if actualProvider == "wema" {
		return s.registerWema(providerCtx, rl, req, reg, requestedFace, bvnRecord)
	}

	rl.step("wallet_generate")
	if walletGenerator == nil {
		rl.Errorf("wallet generator for %s is nil - check wiring in router.go", actualProvider)
		return nil, fmt.Errorf("wallet generation service is not configured")
	}
	walletResp, err := walletGenerator.GenerateWallet(providerCtx, &auth.WalletPayload{
		RequestID:         reg.mobileUserID,
		BVN:               reg.bvn,
		FirstName:         strings.TrimSpace(req.FirstName),
		LastName:          strings.TrimSpace(req.LastName),
		MothersMaidenName: strings.TrimSpace(req.MothersMaidenName),
		DateOfBirth:       reg.dobRaw,
		PhoneNumber:       normalizedPhone,
		Email:             normalizedEmail,
		Address:           reg.address,
		HouseNo:           strings.TrimSpace(req.HouseNo),
		ProductId:         s.optimusProductID,
		Gender:            strings.TrimSpace(req.Gender),
		MaritalStatus:     strings.TrimSpace(req.MaritalStatus),
		Metadata:          map[string]interface{}{},
	})
	if err != nil {
		return nil, err
	}
	if err := s.persistRegistration(providerCtx, rl, reg, walletResp, actualProvider, nil); err != nil {
		return nil, err
	}
	return s.issueSession(providerCtx, rl, reg.mobileUserID, reg.device.DeviceID, ip)
}

// registerDemo completes registration for the whitelisted demo account,
// which already has its user and wallet records: no GenerateWallet call and no
// user/wallet/verification/referral inserts. The device is still bound (if not
// already) so later device-validated requests from the reviewer's device pass.
func (s *Service) registerDemo(ctx context.Context, existingUser *models.User, req OptimusRegisterRequest, ip string) (*RegisterResponse, error) {
	log.Printf("[registerv2.Register] demo account: skipping wallet generation and inserts")

	if s.sessionIssuer == nil {
		return nil, fmt.Errorf("session issuer is not configured")
	}

	deviceRepo := device.NewRepository(s.repo.db)
	if _, findErr := deviceRepo.FindDevice(ctx, existingUser.ID, req.Device.DeviceID); findErr != nil {
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return nil, findErr
		}
		deviceService := device.NewService(*deviceRepo, s.auditLogger)
		if err := deviceService.BindDevice(ctx, existingUser.ID, &device.DeviceBindingRequest{
			DeviceID:    req.Device.DeviceID,
			PublicKey:   req.Device.PublicKey,
			DeviceName:  req.Device.DeviceName,
			DeviceModel: req.Device.DeviceModel,
			OS:          req.Device.OS,
			OSVersion:   req.Device.OSVersion,
			AppVersion:  req.Device.AppVersion,
			IP:          ip,
		}); err != nil {
			return nil, err
		}
	}

	session, err := s.sessionIssuer.IssueSessionTokens(ctx, existingUser.ID, req.Device.DeviceID, ip)
	if err != nil {
		return nil, err
	}
	return &RegisterResponse{
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
	}, nil
}

// RequestPhoneOTP sends an OTP to the given phone number, delegating to the
// same otp.OTPManager the old login/registration flow uses (reusing its OTP
// generation, hashing, expiry, rate-limiting, and SMS dispatch rather than
// reimplementing it). Uses PurposeSubmittedContact rather than PurposeSignup
// so the resulting VerificationRecord keeps Type "phone" instead of being
// stamped with the umbrella "otp" type - Register checks for Type == phone.
func (s *Service) RequestPhoneOTP(ctx context.Context, phone string) (string, error) {
	if s.otpManager == nil {
		return "", fmt.Errorf("otp service is not configured")
	}

	normalized, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(phone))
	if err != nil {
		return "", err
	}

	result, err := s.otpManager.Issue(ctx, otp.IssueOTPInput{
		Purpose:     otp.PurposeSubmittedContact,
		Channel:     otp.ChannelSMS,
		Destination: normalized,
	})
	if err != nil {
		return "", otpError(err)
	}
	return result.OTPID, nil
}

// VerifyPhoneOTP confirms the code sent by RequestPhoneOTP and returns the
// resulting verification ID for use in the final /register call.
func (s *Service) VerifyPhoneOTP(ctx context.Context, otpID, code string) (string, error) {
	if s.otpManager == nil {
		return "", fmt.Errorf("otp service is not configured")
	}

	result, err := s.otpManager.Verify(ctx, otp.VerifyOTPInput{
		Purpose: otp.PurposeSubmittedContact,
		OTPID:   strings.TrimSpace(otpID),
		Code:    strings.TrimSpace(code),
	})
	if err != nil {
		return "", otpError(err)
	}
	return result.VerificationID, nil
}

// RequestEmailOTP sends an OTP to the given email address, delegating to the
// same otp.OTPManager used for phone (see RequestPhoneOTP) but over the email
// channel.
func (s *Service) RequestEmailOTP(ctx context.Context, email string) (string, error) {
	if s.otpManager == nil {
		return "", fmt.Errorf("otp service is not configured")
	}

	normalized := strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized {
		return "", userErrorf("invalid email address")
	}

	result, err := s.otpManager.Issue(ctx, otp.IssueOTPInput{
		Purpose:     otp.PurposeSubmittedContact,
		Channel:     otp.ChannelEmail,
		Destination: normalized,
	})
	if err != nil {
		return "", otpError(err)
	}
	return result.OTPID, nil
}

// VerifyEmailOTP confirms the code sent by RequestEmailOTP and returns the
// resulting verification ID for use in the final /register call.
func (s *Service) VerifyEmailOTP(ctx context.Context, otpID, code string) (string, error) {
	if s.otpManager == nil {
		return "", fmt.Errorf("otp service is not configured")
	}

	result, err := s.otpManager.Verify(ctx, otp.VerifyOTPInput{
		Purpose: otp.PurposeSubmittedContact,
		OTPID:   strings.TrimSpace(otpID),
		Code:    strings.TrimSpace(code),
	})
	if err != nil {
		return "", otpError(err)
	}
	return result.VerificationID, nil
}

// validateBVNWithRetries calls Optimus's validate-bvn up to
// bvnValidateMaxAttempts times, stopping early on the first attempt that
// transports successfully and comes back as an Optimus success. Everything
// else - transport errors, timeouts, and explicit Optimus rejections alike -
// is retried, since a single failure can't tell an incorrect BVN apart from a
// momentary Optimus outage. request.RequestId is reused across attempts so
// they're correlated as the same logical check on Optimus's side.
func (s *Service) validateBVNWithRetries(ctx context.Context, request OptimusBVNValidationRequest) (*OptimusResponse, error) {
	budgetCtx, cancel := context.WithTimeout(ctx, bvnValidateOverallBudget)
	defer cancel()

	var response *OptimusResponse
	var err error
	for attempt := 1; attempt <= bvnValidateMaxAttempts; attempt++ {
		response, err = s.optimus.ValidateBVN(budgetCtx, request)
		if err == nil && response != nil && isOptimusSuccess(response.ResponseCode) {
			return response, nil
		}
		if err == nil {
			err = optimusResponseError(response)
			code := "<nil response>"
			if response != nil {
				code = response.ResponseCode
			}
			log.Printf("[registerv2] level=WARN op=validate_bvn optimus attempt %d/%d rejected: response_code=%q message=%q",
				attempt, bvnValidateMaxAttempts, code, err.Error())
		} else {
			log.Printf("[registerv2] level=WARN op=validate_bvn optimus attempt %d/%d failed: %v", attempt, bvnValidateMaxAttempts, err)
		}
		if attempt == bvnValidateMaxAttempts {
			break
		}
		select {
		case <-budgetCtx.Done():
			return nil, err
		case <-time.After(bvnValidateRetryDelay):
		}
	}
	return nil, err
}

// ValidateBVN kicks off BVN validation for the active provider. When Providus
// is the current preference, it short-circuits immediately: Providus checks
// the BVN itself as part of wallet creation, so there's no separate
// validation call or OTP challenge to run here (requiresOTP comes back
// false, verificationID/providerReferenceID empty). When Optimus is active,
// this behaves as before - a real validate-bvn call that starts an Optimus
// OTP challenge the client must confirm via VerifyOTP (requiresOTP true).
// errWemaNoOTP: Wema registration verifies the customer with face liveness,
// so there is no OTP challenge to verify or resend.
var errWemaNoOTP error = &userError{msg: "wema registration does not use an otp"}

func (s *Service) ValidateBVN(ctx context.Context, request OptimusBVNValidationRequest) (verificationID, providerReferenceID string, requiresOTP bool, err error) {
	rl := newOpLogger("validate_bvn")
	rl.with("bvn", maskIdentityNumber(strings.TrimSpace(request.Bvn))).with("phone", maskPhone(request.PhoneNumber))
	defer func() {
		rl.with("verification_id", verificationID).with("provider_ref", providerReferenceID)
		rl.finish(&err)
	}()

	rl.step("provider_preference")
	provider, err := s.CurrentProvider(ctx)
	if err != nil {
		return "", "", false, err
	}
	rl.with("provider", strings.TrimSpace(provider))
	if strings.TrimSpace(provider) == "providus" {
		rl.Infof("providus validates the bvn at wallet creation - nothing to do here")
		return "", "", false, nil
	}

	if strings.EqualFold(strings.TrimSpace(provider), "wema") || strings.EqualFold(strings.TrimSpace(provider), "alat") {
		if s.alat == nil {
			return "", "", false, fmt.Errorf("wema alat validation service is not configured")
		}
		// Wema registration opens the account with face biometrics (liveness),
		// so the BVN is validated via the active KYC provider (Tendar or
		// Prembly) without an SMS OTP. ALAT's BVN + OTP account creation API
		// is deliberately not used here.
		if s.sessionIssuer == nil {
			return "", "", false, fmt.Errorf("wema bvn validation requires a kyc provider, which is not configured")
		}
		rl.step("kyc_validate_bvn")
		bvnResp, err := s.sessionIssuer.ValidateBVN(ctx, strings.TrimSpace(request.Bvn))
		if err != nil {
			if recordErr := s.createFailedIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, "wema", err); recordErr != nil {
				rl.Errorf("could not save the failed verification record: %v", recordErr)
			}
			return "", "", false, err
		}
		return bvnResp.GetVerificationID(), "", false, nil
	}

	if s.optimus == nil {
		return "", "", false, fmt.Errorf("optimus validation service is not configured")
	}

	// RequestId is our correlation ID for this call, not client input -
	// always generated server-side, overwriting anything the caller sent.
	request.RequestId = uuid.NewString()

	// Retried below before we conclude anything - a single failure here could
	// just as easily be a momentary Optimus blip as a genuinely incorrect BVN,
	// and only the latter should count against this BVN for fallback purposes.
	rl.step("optimus_validate_bvn")
	response, err := s.validateBVNWithRetries(ctx, request)
	if err != nil {
		// Record the failure so Register can tell "validate-bvn was called and
		// consistently failed" (fallback-eligible) apart from "never called"
		// (not fallback-eligible).
		if recordErr := s.createFailedIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, "optimus", err); recordErr != nil {
			rl.Errorf("could not save the failed verification record: %v", recordErr)
		}
		return "", "", false, err
	}

	// response.Data carries Optimus's own reference id for this check, which the
	// client needs later for /otp/verify and /otp/resend - store it as the
	// record's provider verification id rather than our own request.RequestId.
	// Record starts pending, not verified - VerifyOTP flips it to verified
	// once the OTP challenge this call just triggered is actually confirmed,
	// so an unconfirmed/failed OTP correctly falls through to Providus at
	// Register instead of silently counting as an Optimus-verified BVN.
	providerReferenceID = strings.TrimSpace(response.Data)
	verificationID, err = s.createPendingIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, providerReferenceID, "optimus")
	if err != nil {
		return "", "", false, err
	}
	return verificationID, providerReferenceID, true, nil
}

// ValidateBVNWithFace performs face biometric verification against the BVN on record using
// the active KYC provider (Tendar or Prembly) configured in system preferences.
func (s *Service) ValidateBVNWithFace(ctx context.Context, req BVNWithFaceRequest) (faceCheckID string, retErr error) {
	rl := newOpLogger("validate_bvn_with_face")
	rl.with("bvn", maskIdentityNumber(strings.TrimSpace(req.BVN))).with("verification_id", strings.TrimSpace(req.VerificationID))
	defer func() {
		rl.with("face_check", faceCheckID)
		rl.finish(&retErr)
	}()

	if s.sessionIssuer == nil {
		rl.Errorf("session issuer is nil - check wiring in router.go")
		return "", fmt.Errorf("session issuer is not configured")
	}

	verificationID := strings.TrimSpace(req.VerificationID)
	rl.step("bvn_record")

	// Check the BVN record before paying for a face check against it. Two
	// valid states reach here: already verified (Tendar/Prembly
	// /validate/bvn saves records as verified straight away) and pending (the
	// ALAT OTP path, where this face check is what completes verification).
	var record *models.VerificationRecord
	if verificationID != "" && s.repo != nil {
		rec, err := s.repo.GetVerificationByID(ctx, verificationID)
		if err != nil {
			return "", err
		}
		if rec == nil || rec.Type != models.VerificationTypeBVN {
			if rec != nil {
				rl.Warnf("verification record has type %s, want bvn", rec.Type)
			}
			return "", userErrorf("bvn verification record not found")
		}
		rl.Infof("loaded %s", describeBVNRecord(rec))
		if rec.SubjectHash != bvnSubjectHash(req.BVN) {
			rl.Warnf("request bvn does not hash to the record's subject_hash - client sent a different bvn than it validated")
			return "", userErrorf("bvn does not match the verification record")
		}
		switch rec.Status {
		case models.VerificationStatusVerified, models.VerificationStatusPending:
		case models.VerificationStatusUsed:
			return "", userErrorf("this bvn has already been used for registration")
		default:
			return "", userErrorf("bvn verification failed, please validate again")
		}
		if isExpired(rec.ExpiresAt, time.Now().UTC()) {
			return "", userErrorf("bvn verification has expired, please validate again")
		}
		record = rec
	}

	rl.step("face_provider")
	info, err := s.sessionIssuer.ValidateBVNWithFace(ctx, auth.BVNWithFaceValidationRequest{
		VerificationID: req.VerificationID,
		BVN:            req.BVN,
		Image:          req.Image,
	})
	if err != nil {
		return "", err
	}

	rl.step("mark_verified")

	// A pending record becomes verified now that the face check passed, so
	// Register recognizes it. An already-verified record needs no update -
	// MarkVerificationVerified deliberately only flips pending records.
	if record != nil && record.Status == models.VerificationStatusPending {
		if err := s.repo.MarkVerificationVerified(ctx, record.ID, time.Now().UTC()); err != nil {
			return "", fmt.Errorf("failed to mark verification record verified: %w", err)
		}
		rl.Infof("pending bvn record marked verified by face check")
	}

	return info.FaceCheckID, nil
}

// bvnSubjectHash is the SubjectHash VerificationRecords carry for a BVN.
func bvnSubjectHash(bvn string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(bvn)))
	return hex.EncodeToString(sum[:])
}

func isExpired(expiresAt *time.Time, now time.Time) bool {
	return expiresAt != nil && !expiresAt.After(now)
}

// resolveBVNRecord picks the BVN verification record Register should use.
// When the client names a face check, the record that face check was taken
// against wins, provided it's still a valid verified record for this same BVN
// - otherwise a second /validate/bvn call would leave the face check pointing
// at an older record than the latest-by-hash lookup returns, and Register
// would wrongly report the face check as missing. Falls back to the latest
// record for the BVN. The face check record named by the client (if found) is
// returned too, so the caller doesn't have to load it again.
func (s *Service) resolveBVNRecord(ctx context.Context, bvn, faceVerificationID string) (*models.VerificationRecord, *models.FaceCheckRecord, error) {
	hash := bvnSubjectHash(bvn)

	var face *models.FaceCheckRecord
	if id := strings.TrimSpace(faceVerificationID); id != "" {
		fc, err := s.repo.GetFaceCheckRecord(ctx, id)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
		if err == nil && fc != nil {
			face = fc
			rec, err := s.repo.GetVerificationByID(ctx, fc.VerificationRecordID)
			if err != nil {
				return nil, nil, err
			}
			if rec != nil &&
				rec.Type == models.VerificationTypeBVN &&
				rec.SubjectHash == hash &&
				rec.Status == models.VerificationStatusVerified &&
				!isExpired(rec.ExpiresAt, time.Now().UTC()) {
				return rec, face, nil
			}
		}
	}

	rec, err := s.repo.GetLatestByTypeAndHash(ctx, models.VerificationTypeBVN, hash)
	if err != nil {
		return nil, nil, err
	}
	return rec, face, nil
}

// checkBVNRecordUsable turns used and expired BVN records into specific
// errors, instead of the generic "otp not complete"/"not verified" messages
// they'd otherwise fall through to. A nil record is left to the caller.
func checkBVNRecordUsable(rec *models.VerificationRecord) error {
	if rec == nil {
		return nil
	}
	switch {
	case rec.Status == models.VerificationStatusUsed:
		return userErrorf("this bvn has already been used for registration")
	case rec.Status == models.VerificationStatusVerified && isExpired(rec.ExpiresAt, time.Now().UTC()):
		return userErrorf("bvn verification has expired, please validate again")
	}
	return nil
}

// resolveMatchedFaceRecord returns a matched face check taken against
// bvnRecord: the one the client named if it qualifies, otherwise the latest
// one recorded for bvnRecord. Returns nil (no error) when none qualifies.
func (s *Service) resolveMatchedFaceRecord(ctx context.Context, requested *models.FaceCheckRecord, bvnRecord *models.VerificationRecord) (*models.FaceCheckRecord, error) {
	if bvnRecord == nil {
		return nil, nil
	}
	if requested != nil && requested.VerificationRecordID == bvnRecord.ID && requested.Matched {
		return requested, nil
	}
	fc, err := s.repo.GetFaceCheckRecordByVerificationID(ctx, bvnRecord.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if fc == nil || !fc.Matched {
		return nil, nil
	}
	return fc, nil
}

// alatAccountMatchesApplicant reports whether an existing ALAT account belongs
// to this applicant. ALAT names the account from the BVN, so its names are
// matched against the BVN record's name - not what the customer typed, which
// would strand a retry whenever the typed name differs from the BVN (e.g.
// "Bayo" vs "Adebayo") after ALAT had already created the account. Email is
// not compared: it's a contact detail the customer may change between
// attempts. Without a BVN name (e.g. Optimus records), it falls back to the
// submitted names. Fields ALAT leaves empty aren't held against the applicant.
func alatAccountMatchesApplicant(d *AlatAccountDetails, bvnRecord *models.VerificationRecord, firstName, lastName string) bool {
	if d == nil {
		return false
	}
	if bvnName := verifiedName(bvnRecord); bvnName != "" {
		return nameTokensWithin(bvnName, d.FirstName, d.LastName)
	}
	if v := normalizeName(d.FirstName); v != "" && v != normalizeName(firstName) {
		return false
	}
	if v := normalizeName(d.LastName); v != "" && v != normalizeName(lastName) {
		return false
	}
	return true
}

// checkApplicantMatchesBVN rejects a registration whose first name, last name
// or date of birth differ from the verified BVN. Names match when every word
// of the submitted first and last name appears in the BVN name, so word order
// and an omitted middle name don't count against the customer.
func checkApplicantMatchesBVN(bvnRecord *models.VerificationRecord, firstName, lastName, dob string) error {
	if bvnName := verifiedName(bvnRecord); bvnName != "" {
		if !nameTokensWithin(bvnName, firstName, lastName) {
			return userErrorf("first and last name must match the name on your BVN")
		}
	}
	if bvnRecord != nil && bvnRecord.VerifiedDOB != nil && strings.TrimSpace(*bvnRecord.VerifiedDOB) != "" {
		if !auth.DOBsMatch(*bvnRecord.VerifiedDOB, dob) {
			return userErrorf("date of birth must match the date of birth on your BVN")
		}
	}
	return nil
}

func verifiedName(rec *models.VerificationRecord) string {
	if rec == nil || rec.VerifiedName == nil {
		return ""
	}
	return strings.TrimSpace(*rec.VerifiedName)
}

func bvnRecordID(rec *models.VerificationRecord) string {
	if rec == nil {
		return "-"
	}
	return rec.ID
}

// nameTokensWithin reports whether every word in names appears in full, and
// at least one word is given. Comparison ignores case and punctuation.
func nameTokensWithin(full string, names ...string) bool {
	have := map[string]bool{}
	for _, t := range nameTokens(full) {
		have[t] = true
	}
	n := 0
	for _, name := range names {
		for _, t := range nameTokens(name) {
			if !have[t] {
				return false
			}
			n++
		}
	}
	return n > 0
}

func nameTokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) })
}

func normalizeName(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// GetAddressDropDownList calls ALAT Step 0 to retrieve address dropdown choices.
func (s *Service) GetAddressDropDownList(ctx context.Context) (any, error) {
	if s.alat == nil {
		return nil, fmt.Errorf("wema alat service is not configured")
	}
	return s.alat.GetAddressDropDownList(ctx)
}

// VerifyOTP confirms the OTP Optimus/ALAT sent for referenceID and, on success,
// flips the matching pending BVN/NIN verification record to verified. Generic
// - works for any Optimus/ALAT OTP challenge, since the reference id alone is
// what the provider uses to identify which challenge this is.
func (s *Service) VerifyOTP(ctx context.Context, phone, otpToken, email, referenceID string) (retErr error) {
	rl := newOpLogger("verify_otp")
	rl.with("phone", maskPhone(phone)).with("provider_ref", strings.TrimSpace(referenceID))
	defer rl.finish(&retErr)

	rl.step("resolve_provider")
	provider := s.providerForReference(ctx, rl, referenceID)
	if provider == "" {
		p, err := s.CurrentProvider(ctx)
		if err != nil {
			return err
		}
		provider = p
	}
	rl.with("provider", provider)
	rl.step("provider_verify")

	if strings.EqualFold(provider, "wema") || strings.EqualFold(provider, "alat") {
		return errWemaNoOTP
	}

	if s.optimus == nil {
		return fmt.Errorf("optimus validation service is not configured")
	}
	if err := s.optimus.VerifyOTPWithOptimus(ctx, phone, otpToken, email, referenceID); err != nil {
		return err
	}
	if s.repo != nil {
		rl.step("mark_verified")
		return s.repo.MarkVerificationVerifiedByProviderReference(ctx, strings.TrimSpace(referenceID), time.Now().UTC())
	}
	return nil
}

// ResendOTP asks Optimus or ALAT to resend the OTP tied to referenceID, returning the
// new reference id the caller must use for the next verify/resend. Also
// repoints the pending verification record at the new reference id.
func (s *Service) ResendOTP(ctx context.Context, referenceID string) (newRef string, retErr error) {
	rl := newOpLogger("resend_otp")
	rl.with("provider_ref", strings.TrimSpace(referenceID))
	defer func() {
		rl.with("new_provider_ref", newRef)
		rl.finish(&retErr)
	}()

	rl.step("resolve_provider")
	provider := s.providerForReference(ctx, rl, referenceID)
	if provider == "" {
		p, err := s.CurrentProvider(ctx)
		if err != nil {
			return "", err
		}
		provider = p
	}
	rl.with("provider", provider)
	rl.step("provider_resend")

	if strings.EqualFold(provider, "wema") || strings.EqualFold(provider, "alat") {
		return "", errWemaNoOTP
	}

	if s.optimus == nil {
		return "", fmt.Errorf("optimus validation service is not configured")
	}
	newReferenceID, err := s.optimus.ResendOTPWithOptimus(ctx, referenceID)
	if err != nil {
		return "", err
	}
	if s.repo != nil {
		if err := s.repo.UpdateProviderVerificationReference(ctx, strings.TrimSpace(referenceID), strings.TrimSpace(newReferenceID)); err != nil {
			return "", err
		}
	}
	return newReferenceID, nil
}

// providerForReference returns the provider that issued the OTP challenge
// referenceID belongs to, or "" (caller falls back to the current
// preference). Lookup errors are logged rather than returned: falling back is
// still the right behaviour, but a DB error here should be visible.
func (s *Service) providerForReference(ctx context.Context, rl *opLogger, referenceID string) string {
	if s.repo == nil {
		return ""
	}
	rec, err := s.repo.GetVerificationByProviderReference(ctx, referenceID)
	switch {
	case err != nil:
		rl.Warnf("verification lookup by provider reference failed, falling back to current provider preference: %v", err)
		return ""
	case rec == nil:
		rl.Warnf("no verification record for this provider reference, falling back to current provider preference")
		return ""
	}
	rl.with("verification_id", rec.ID)
	return strings.TrimSpace(rec.Provider)
}

// createFailedIdentityVerification records that Optimus's validate-bvn/nin
// call itself rejected identityNumber (as opposed to it never being called at
// all, or being called but never completing its OTP challenge). Register uses
// this to know a Providus fallback is actually warranted.
func (s *Service) createFailedIdentityVerification(ctx context.Context, verificationType, identityNumber, provider string, failureErr error) error {
	now := time.Now().UTC()
	hash := sha256.Sum256([]byte(identityNumber))
	masked := maskIdentityNumber(identityNumber)
	reason := failureErr.Error()

	record := &models.VerificationRecord{
		ID:            uuid.NewString(),
		Type:          verificationType,
		Provider:      provider,
		Status:        models.VerificationStatusFailed,
		SubjectHash:   hex.EncodeToString(hash[:]),
		SubjectMasked: &masked,
		FailureReason: &reason,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return s.repo.AddVerification(ctx, record)
}

// createPendingIdentityVerification records a BVN/NIN check that has passed
// the provider's initial validate call but not yet its OTP challenge. Status stays
// pending - and therefore invisible to Register's verified-record lookup -
// until VerifyOTP confirms the OTP.
func (s *Service) createPendingIdentityVerification(ctx context.Context, verificationType, identityNumber, providerReferenceID, provider string) (string, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(verificationExpiry)
	hash := sha256.Sum256([]byte(identityNumber))
	masked := maskIdentityNumber(identityNumber)

	record := &models.VerificationRecord{
		ID:                     uuid.NewString(),
		Type:                   verificationType,
		Provider:               provider,
		Status:                 models.VerificationStatusPending,
		SubjectHash:            hex.EncodeToString(hash[:]),
		SubjectMasked:          &masked,
		ProviderVerificationID: stringPointer(strings.TrimSpace(providerReferenceID)),
		VerifiedID:             &identityNumber,
		ExpiresAt:              &expiresAt,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := s.repo.AddVerification(ctx, record); err != nil {
		return "", err
	}
	return record.ID, nil
}

func isOptimusSuccess(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" || strings.EqualFold(code, "00") || strings.EqualFold(code, "0") || strings.EqualFold(code, "success") {
		return true
	}
	status, err := strconv.Atoi(code)
	return err == nil && status >= 200 && status < 300
}

func optimusResponseError(response *OptimusResponse) error {
	if response == nil {
		return userErrorf("we could not verify your details right now. Please try again")
	}
	if message := strings.TrimSpace(response.ResponseMessage); message != "" {
		return userErrorf("%s", message)
	}
	if response.Error != nil {
		return userErrorf("%v", response.Error)
	}
	return userErrorf("we could not verify your details. Please check them and try again")
}

func maskIdentityNumber(value string) string {
	if len(value) <= 4 {
		return value
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 1 {
		return "***"
	}
	return email[:1] + strings.Repeat("*", at-1) + email[at:]
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
