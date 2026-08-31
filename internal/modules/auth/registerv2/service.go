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
	"neat_mobile_app_backend/internal/modules/auth"
	"neat_mobile_app_backend/internal/modules/auth/otp"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/modules/referrals"
	"neat_mobile_app_backend/internal/modules/wallet"
	phoneutil "neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/models"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const verificationExpiry = 30 * time.Minute

// recoverWalletByPhoneBackoff is the delay schedule recoverWalletByPhone
// retries on - a var (not a const) so tests can shrink it instead of
// sleeping through the real delays. Runs on the detached providerCtx, not
// the client's connection, so being generous here is cheap - it only trades
// off how long a genuinely failed registration takes to give up, not
// perceived request latency for the client (who's very likely already gone
// by the time GenerateWallet itself has failed).
var recoverWalletByPhoneBackoff = []time.Duration{0, 5 * time.Second, 15 * time.Second, 30 * time.Second}

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
	}
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
func (s *Service) Register(ctx context.Context, req OptimusRegisterRequest, ip string) (*RegisterResponse, error) {
	if err := authchecker.ValidatePassword(req.Password); err != nil {
		return nil, err
	}
	if req.Password != req.ConfirmPassword {
		return nil, fmt.Errorf("password and confirm password do not match")
	}
	if err := authchecker.ValidatePin(req.TransactionPin); err != nil {
		return nil, err
	}
	if req.TransactionPin != req.ConfirmTransactionPin {
		return nil, fmt.Errorf("transaction pin and confirm transaction pin do not match")
	}

	normalizedPhone, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(req.PhoneNumber))
	if err != nil {
		return nil, err
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))

	phoneRecord, err := s.repo.GetVerificationByID(ctx, strings.TrimSpace(req.PhoneVerificationID))
	if err != nil {
		return nil, err
	}
	if phoneRecord == nil || phoneRecord.Status != models.VerificationStatusVerified ||
		phoneRecord.Type != models.VerificationTypePhone ||
		phoneRecord.VerifiedPhone == nil || strings.TrimSpace(*phoneRecord.VerifiedPhone) != normalizedPhone {
		return nil, fmt.Errorf("phone number is not verified")
	}

	emailRecord, err := s.repo.GetVerificationByID(ctx, strings.TrimSpace(req.EmailVerificationID))
	if err != nil {
		return nil, err
	}
	if emailRecord == nil || emailRecord.Status != models.VerificationStatusVerified ||
		emailRecord.Type != models.VerificationTypeEmail ||
		emailRecord.VerifiedEmail == nil || strings.ToLower(strings.TrimSpace(*emailRecord.VerifiedEmail)) != normalizedEmail {
		return nil, fmt.Errorf("email is not verified")
	}

	// NIN is collected and persisted below but no longer gates registration -
	// it isn't sent to either provider's wallet-creation call, so validating
	// it up front only added friction without changing the outcome.

	// A local user for this phone means registration already completed here
	// previously - fail fast with a clear error rather than calling the
	// wallet provider again, which would just be rejected as a duplicate.
	authRepo := auth.NewRespository(s.repo.db, s.repo.cipher)
	if _, lookupErr := authRepo.GetUserByPhone(ctx, normalizedPhone); lookupErr == nil {
		return nil, appErr.ErrUserExists
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return nil, lookupErr
	}

	provider, err := s.CurrentProvider(ctx)
	if err != nil {
		return nil, err
	}

	// bvnRecord is only set (and later marked used) when Optimus actually
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
			log.Println("optimus service is not configured")
			return nil, fmt.Errorf("optimus service is not configured")
		}

		bvnHash := sha256.Sum256([]byte(strings.TrimSpace(req.BVN)))
		latestBVNRecord, err := s.repo.GetLatestByTypeAndHash(ctx, models.VerificationTypeBVN, hex.EncodeToString(bvnHash[:]))
		if err != nil {
			return nil, err
		}
		switch {
		case latestBVNRecord == nil:
			// /auth/validate/bvn was never called for this BVN - fallback only
			// applies to calls that were made and Optimus rejected, so this is
			// a hard stop, not a fallback trigger.
			return nil, fmt.Errorf("bvn is not verified")

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

		default:
			// Pending: validate-bvn succeeded but the OTP challenge was never
			// confirmed (or failed). This is not a fallback case - falling back
			// here would let someone route around the OTP consent step just by
			// not completing it, so the user must resolve it instead.
			return nil, fmt.Errorf("bvn otp verification is not complete")
		}

	default:
		return nil, fmt.Errorf("registration provider %q is not supported by this flow", provider)
	}
	if walletGenerator == nil {
		return nil, fmt.Errorf("wallet generation service is not configured")
	}

	dob, err := time.Parse("2006-01-02", strings.TrimSpace(req.Dob))
	if err != nil {
		return nil, fmt.Errorf("invalid date of birth: %w", err)
	}

	mobileUserID := uuid.NewString()
	internalWalletID := uuid.NewString()

	// We're about to trigger an external, irreversible side effect (wallet
	// creation at the provider) - from here on, work must not be abortable
	// just because the client disconnects (c.Request.Context() is canceled
	// when that happens). A canceled ctx during this window previously left
	// the provider account orphaned with nothing saved locally: the
	// wallet-generation HTTP call itself can time out client-side (observed
	// in production at just over its own client timeout) while the provider
	// keeps processing and completes the account anyway, and any subsequent
	// work on the original ctx would fail immediately once the client gave
	// up. providerCtx keeps request-scoped values but drops the parent's
	// cancellation, giving the provider call, the recovery lookup below, the
	// DB write, and session issuance a guaranteed bounded runway of their own
	// instead. 6 minutes comfortably covers the pathological worst case
	// (GenerateWallet's own 60s timeout, plus recoverWalletByPhoneBackoff's
	// retries each doing up to two lookup calls at 30s apiece) without being
	// unbounded - actual completion is almost always much faster.
	providerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 6*time.Minute)
	defer cancel()

	walletResp, err := walletGenerator.GenerateWallet(providerCtx, &auth.WalletPayload{
		RequestID:         mobileUserID,
		BVN:               strings.TrimSpace(req.BVN),
		FirstName:         strings.TrimSpace(req.FirstName),
		LastName:          strings.TrimSpace(req.LastName),
		MothersMaidenName: strings.TrimSpace(req.MothersMaidenName),
		DateOfBirth:       strings.TrimSpace(req.Dob),
		PhoneNumber:       normalizedPhone,
		Email:             normalizedEmail,
		Address:           strings.TrimSpace(req.Address),
		HouseNo:           strings.TrimSpace(req.HouseNo),
		ProductId:         s.optimusProductID,
		Gender:            strings.TrimSpace(req.Gender),
		MaritalStatus:     strings.TrimSpace(req.MaritalStatus),
		Metadata:          map[string]interface{}{},
	})
	if err != nil {
		// A failure here is ambiguous - it may mean the provider never
		// received the request, or it may mean our client gave up waiting
		// while the provider kept processing and completed it anyway. Recover
		// by phone number, which we always know up front - unlike the
		// provider's own customerId, which we'd otherwise have no way to
		// discover after a true timeout (the provider doesn't echo back
		// anything we send as its customerId).
		if recovered := s.recoverWalletByPhone(providerCtx, walletGenerator, normalizedPhone); recovered != nil {
			walletResp, err = recovered, nil
		}
	}
	if err != nil {
		return nil, err
	}
	if walletResp == nil || walletResp.Customer == nil || walletResp.Wallet == nil {
		return nil, fmt.Errorf("wallet provider returned an incomplete response")
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	pinHash, err := auth.HashPassword(req.TransactionPin)
	if err != nil {
		return nil, err
	}

	address := strings.TrimSpace(req.Address)
	if walletResp.Customer.Address != nil && strings.TrimSpace(*walletResp.Customer.Address) != "" {
		address = strings.TrimSpace(*walletResp.Customer.Address)
	}

	capExpiresAt := time.Now().UTC().Add(24 * time.Hour)
	addressCopy := address
	emailCopy := normalizedEmail
	user := &models.User{
		ID:                     mobileUserID,
		WalletID:               internalWalletID,
		Phone:                  normalizedPhone,
		Address:                &addressCopy,
		Email:                  &emailCopy,
		FirstName:              req.FirstName,
		LastName:               req.LastName,
		PasswordHash:           passwordHash,
		PinHash:                pinHash,
		DOB:                    dob,
		BVN:                    strings.TrimSpace(req.BVN),
		NIN:                    strings.TrimSpace(req.NIN),
		IsEmailVerified:        true,
		IsPhoneVerified:        true,
		IsBvnVerified:          true,
		IsNinVerified:          true,
		IsBiometricsEnabled:    *req.IsBiomtricsEnabled,
		IsNotificationsEnabled: true,
		ActivationCapAmount:    s.activationCapKobo,
		ActivationCapExpiresAt: &capExpiresAt,
	}

	walletRecord := &wallet.CustomerWallet{
		ID:               uuid.NewString(),
		InternalWalletID: internalWalletID,
		MobileUserID:     mobileUserID,
		PhoneNumber:      walletResp.Customer.PhoneNumber,
		WalletCustomerID: walletResp.Customer.ID,
		Metadata:         walletResp.Customer.Metadata,
		BVN:              walletResp.Customer.BVN,
		Currency:         walletResp.Customer.Currency,
		DateOfBirth:      walletResp.Customer.DateOfBirth,
		FirstName:        walletResp.Customer.FirstName,
		LastName:         walletResp.Customer.LastName,
		Email:            walletResp.Customer.Email,
		Address:          address,
		MerchantID:       walletResp.Customer.MerchantId,
		Tier:             walletResp.Customer.Tier,
		WalletID:         walletResp.Wallet.WalletId,
		Mode:             walletResp.Customer.Mode,
		BankName:         walletResp.Wallet.BankName,
		BankCode:         walletResp.Wallet.BankCode,
		Provider:         actualProvider,
		AccountNumber:    walletResp.Wallet.AccountNumber,
		AccountName:      walletResp.Wallet.AccountName,
		AccountRef:       walletResp.Wallet.AccountReference,
		BookedBalance:    walletResp.Wallet.BookedBalance,
		AvailableBalance: walletResp.Wallet.AvailableBalance,
		Status:           walletResp.Wallet.Status,
		WalletType:       walletResp.Wallet.WalletType,
		Updated:          walletResp.Wallet.Updated,
		CreatedAt:        time.Now().UTC(),
	}

	now := time.Now().UTC()
	usedVerificationIDs := []string{phoneRecord.ID, emailRecord.ID}
	if bvnRecord != nil {
		usedVerificationIDs = append(usedVerificationIDs, bvnRecord.ID)
	}

	err = s.tx.WithTx(providerCtx, func(txDB *gorm.DB) error {
		authRepo := auth.NewRespository(txDB, s.repo.cipher)
		walletRepo := wallet.NewRepository(txDB)
		deviceRepo := device.NewRepository(txDB)
		verificationRepo := NewRepository(txDB, s.repo.cipher)

		if _, txErr := authRepo.CreateUser(providerCtx, user); txErr != nil {
			return txErr
		}
		if txErr := authRepo.LinkBVNRecordToUser(providerCtx, user.BVN, user.ID); txErr != nil {
			return txErr
		}
		if txErr := walletRepo.CreateWallet(providerCtx, walletRecord); txErr != nil {
			return txErr
		}

		deviceService := device.NewService(*deviceRepo)
		if txErr := deviceService.BindDevice(providerCtx, mobileUserID, &device.DeviceBindingRequest{
			DeviceID:    req.Device.DeviceID,
			PublicKey:   req.Device.PublicKey,
			DeviceName:  req.Device.DeviceName,
			DeviceModel: req.Device.DeviceModel,
			OS:          req.Device.OS,
			OSVersion:   req.Device.OSVersion,
			AppVersion:  req.Device.AppVersion,
			IP:          ip,
		}); txErr != nil {
			return txErr
		}

		for _, id := range usedVerificationIDs {
			if id == "" {
				continue
			}
			if txErr := verificationRepo.MarkVerificationUsed(providerCtx, id, now); txErr != nil {
				return txErr
			}
		}

		referralsService := referrals.NewService(referrals.NewRepository(txDB))

		if strings.TrimSpace(req.ReferralCode) != "" {
			if txErr := referralsService.RedeemReferralCode(providerCtx, mobileUserID, req.ReferralCode); txErr != nil {
				return txErr
			}
		}

		if txErr := referralsService.GenerateAndAssignReferralCode(providerCtx, mobileUserID); txErr != nil {
			return txErr
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.sessionIssuer == nil {
		return nil, fmt.Errorf("session issuer is not configured")
	}
	session, err := s.sessionIssuer.IssueSessionTokens(providerCtx, mobileUserID, req.Device.DeviceID, ip)
	if err != nil {
		return nil, err
	}

	return &RegisterResponse{
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
	}, nil
}

// recoverWalletByPhone checks whether the wallet provider actually completed
// wallet creation despite GenerateWallet returning an error to us - a
// client-side timeout only means we stopped waiting, not that the provider
// didn't finish. Phone number is always known up front, unlike the
// provider's own customerId, so it's used to discover that id first
// (LookupCustomerByPhone), then fetch the full wallet details by it
// (LookupWalletByCustomerID). Retries with a short backoff since the
// provider's own processing may still be finishing right after our client
// gives up waiting. Returns nil if nothing could be recovered, in which case
// the caller should surface the original GenerateWallet error.
func (s *Service) recoverWalletByPhone(ctx context.Context, walletGenerator auth.WalletService, phone string) *auth.WalletResponse {
	for i, delay := range recoverWalletByPhoneBackoff {
		if delay > 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
		}

		customerID, found, err := walletGenerator.LookupCustomerByPhone(ctx, phone)
		if err != nil {
			log.Printf("registerv2: phone-based wallet recovery lookup failed (attempt %d) phone=%s: %v", i+1, phone, err)
			continue
		}
		if !found {
			continue
		}

		recovered, ok, err := walletGenerator.LookupWalletByCustomerID(ctx, customerID)
		if err != nil {
			log.Printf("registerv2: wallet lookup by recovered customer id failed customer_id=%s: %v", customerID, err)
			continue
		}
		if ok && recovered != nil {
			return recovered
		}
	}
	return nil
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
		return "", err
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
		return "", err
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
		return "", fmt.Errorf("invalid email address")
	}

	result, err := s.otpManager.Issue(ctx, otp.IssueOTPInput{
		Purpose:     otp.PurposeSubmittedContact,
		Channel:     otp.ChannelEmail,
		Destination: normalized,
	})
	if err != nil {
		return "", err
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
		return "", err
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
func (s *Service) ValidateBVN(ctx context.Context, request OptimusBVNValidationRequest) (verificationID, providerReferenceID string, requiresOTP bool, err error) {
	provider, err := s.CurrentProvider(ctx)
	if err != nil {
		return "", "", false, err
	}
	if strings.TrimSpace(provider) == "providus" {
		return "", "", false, nil
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
	response, err := s.validateBVNWithRetries(ctx, request)
	if err != nil {
		// Record the failure so Register can tell "validate-bvn was called and
		// consistently failed" (fallback-eligible) apart from "never called"
		// (not fallback-eligible).
		if recordErr := s.createFailedIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, err); recordErr != nil {
			log.Printf("ValidateBVN: failed to record failed verification: %v", recordErr)
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
	verificationID, err = s.createPendingIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, providerReferenceID)
	if err != nil {
		return "", "", false, err
	}
	return verificationID, providerReferenceID, true, nil
}

func (s *Service) ValidateNIN(ctx context.Context, request OptimusNINValidationRequest) (string, string, error) {
	if s.optimus == nil {
		return "", "", fmt.Errorf("optimus validation service is not configured")
	}

	// RequestId is our correlation ID for this call, not client input -
	// always generated server-side, overwriting anything the caller sent.
	request.RequestId = uuid.NewString()

	response, err := s.optimus.ValidateNIN(ctx, request)
	if err != nil {
		return "", "", err
	}
	if response == nil || !isOptimusSuccess(response.ResponseCode) {
		return "", "", optimusResponseError(response)
	}

	// See the matching comment in ValidateBVN: record starts pending, not
	// verified, until VerifyOTP confirms the OTP challenge this call triggered.
	providerReferenceID := strings.TrimSpace(response.Data)
	verificationID, err := s.createPendingIdentityVerification(ctx, models.VerificationTypeNIN, request.Nin, providerReferenceID)
	if err != nil {
		return "", "", err
	}
	return verificationID, providerReferenceID, nil
}

// VerifyOTP confirms the OTP Optimus sent for referenceID and, on success,
// flips the matching pending BVN/NIN verification record to verified. Generic
// - works for any Optimus OTP challenge (BVN/NIN validation today), since the
// reference id alone is what Optimus uses to identify which challenge this is.
func (s *Service) VerifyOTP(ctx context.Context, phone, otpToken, email, referenceID string) error {
	if s.optimus == nil {
		return fmt.Errorf("optimus validation service is not configured")
	}
	if err := s.optimus.VerifyOTPWithOptimus(ctx, phone, otpToken, email, referenceID); err != nil {
		return err
	}
	return s.repo.MarkVerificationVerifiedByProviderReference(ctx, strings.TrimSpace(referenceID), time.Now().UTC())
}

// ResendOTP asks Optimus to resend the OTP tied to referenceID, returning the
// new reference id the caller must use for the next verify/resend. Also
// repoints the pending verification record at the new reference id - without
// this, VerifyOTP's later lookup (by provider reference id) would miss the
// record entirely, since the client is required to verify against the new id.
func (s *Service) ResendOTP(ctx context.Context, referenceID string) (string, error) {
	if s.optimus == nil {
		return "", fmt.Errorf("optimus validation service is not configured")
	}
	newReferenceID, err := s.optimus.ResendOTPWithOptimus(ctx, referenceID)
	if err != nil {
		return "", err
	}
	if err := s.repo.UpdateProviderVerificationReference(ctx, strings.TrimSpace(referenceID), strings.TrimSpace(newReferenceID)); err != nil {
		return "", err
	}
	return newReferenceID, nil
}

// createFailedIdentityVerification records that Optimus's validate-bvn/nin
// call itself rejected identityNumber (as opposed to it never being called at
// all, or being called but never completing its OTP challenge). Register uses
// this to know a Providus fallback is actually warranted.
func (s *Service) createFailedIdentityVerification(ctx context.Context, verificationType, identityNumber string, failureErr error) error {
	now := time.Now().UTC()
	hash := sha256.Sum256([]byte(identityNumber))
	masked := maskIdentityNumber(identityNumber)
	reason := failureErr.Error()

	record := &models.VerificationRecord{
		ID:            uuid.NewString(),
		Type:          verificationType,
		Provider:      "optimus",
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
// Optimus's initial validate call but not yet its OTP challenge. Status stays
// pending - and therefore invisible to Register's verified-record lookup -
// until VerifyOTP confirms the OTP.
func (s *Service) createPendingIdentityVerification(ctx context.Context, verificationType, identityNumber, providerReferenceID string) (string, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(verificationExpiry)
	hash := sha256.Sum256([]byte(identityNumber))
	masked := maskIdentityNumber(identityNumber)

	record := &models.VerificationRecord{
		ID:                     uuid.NewString(),
		Type:                   verificationType,
		Provider:               "optimus",
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
		return fmt.Errorf("we could not verify your details right now. Please try again")
	}
	if message := strings.TrimSpace(response.ResponseMessage); message != "" {
		return fmt.Errorf("%s", message)
	}
	if response.Error != nil {
		return fmt.Errorf("%v", response.Error)
	}
	return fmt.Errorf("we could not verify your details. Please check them and try again")
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
