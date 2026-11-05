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

	var latestBVNRecord *models.VerificationRecord
	if strings.TrimSpace(provider) != "providus" {
		bvnHash := sha256.Sum256([]byte(strings.TrimSpace(req.BVN)))
		rec, err := s.repo.GetLatestByTypeAndHash(ctx, models.VerificationTypeBVN, hex.EncodeToString(bvnHash[:]))
		if err != nil {
			return nil, err
		}
		latestBVNRecord = rec
	}

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
			log.Println("optimus service is not configured")
			return nil, fmt.Errorf("optimus service is not configured")
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

	case "wema", "alat":
		if s.alat == nil {
			log.Println("wema alat service is not configured")
			return nil, fmt.Errorf("wema alat service is not configured")
		}

		switch {
		case latestBVNRecord == nil:
			return nil, fmt.Errorf("bvn is not verified")
		case latestBVNRecord.Status == models.VerificationStatusVerified:
			bvnRecord = latestBVNRecord
			actualProvider = "wema"
		case latestBVNRecord.Status == models.VerificationStatusFailed:
			return nil, fmt.Errorf("bvn verification failed")
		default:
			return nil, fmt.Errorf("bvn otp verification is not complete")
		}

	default:
		return nil, fmt.Errorf("registration provider %q is not supported by this flow", provider)
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

	var walletResp *auth.WalletResponse
	if actualProvider == "wema" {
		var acctDetails *AlatAccountDetails

		// 1. Check if the account already exists on Wema (e.g. from an earlier attempt whose DB write failed/timed out).
		// Reusing the existing account prevents ALAT duplicate errors on retry.
		existingDetails, fetchErr := s.alat.GetPartnershipAccountDetails(providerCtx, normalizedPhone)
		if fetchErr == nil && existingDetails != nil && strings.TrimSpace(existingDetails.AccountNumber) != "" {
			acctDetails = existingDetails
			log.Printf("[ALAT] Found existing account %s for phone %s, skipping Step 1c creation", existingDetails.AccountNumber, normalizedPhone)
		} else {
			// 2. Resolve and verify face check record.
			// Security: live image MUST come from an authoritative, matched FaceCheckRecord linked to this BVN.
			// Client-supplied raw images or unverified/mismatched face checks are rejected.
			var faceRecord *models.FaceCheckRecord
			if strings.TrimSpace(req.BVNWithFaceVerificationID) != "" {
				if fc, err := s.repo.GetFaceCheckRecord(ctx, req.BVNWithFaceVerificationID); err == nil && fc != nil {
					if bvnRecord != nil && fc.VerificationRecordID == bvnRecord.ID && fc.Matched {
						faceRecord = fc
					}
				}
			}
			if faceRecord == nil && bvnRecord != nil {
				if fc, err := s.repo.GetFaceCheckRecordByVerificationID(ctx, bvnRecord.ID); err == nil && fc != nil && fc.Matched {
					faceRecord = fc
				}
			}

			if faceRecord == nil || !faceRecord.Matched || faceRecord.FaceImageProvided == nil || strings.TrimSpace(*faceRecord.FaceImageProvided) == "" {
				return nil, fmt.Errorf("face biometric verification is required for wema registration and must be verified")
			}

			liveImage := strings.TrimSpace(*faceRecord.FaceImageProvided)

			// 3. Invoke Step 1c (Tier 2 Face Account Creation) without residential address
			tier2Req := AlatTier2FaceRequest{
				BVN:                strings.TrimSpace(req.BVN),
				NIN:                strings.TrimSpace(req.NIN),
				PhoneNumber:        normalizedPhone,
				EmailAddress:       normalizedEmail,
				ResidentialAddress: nil, // Omitted for Tier 2; address is added when upgrading to Tier 3
				LiveImageOfFace:    liveImage,
				CorrelationID:      uuid.NewString(),
			}

			trackingID, tier2Err := s.alat.PostTier2PartnershipAccountWithFace(providerCtx, tier2Req)
			if tier2Err != nil {
				log.Printf("[ALAT] Step 1c (Tier 2 Face Account Creation) failed for %s: %v", normalizedPhone, tier2Err)
				return nil, fmt.Errorf("wema alat tier 2 account creation failed: %w", tier2Err)
			}
			log.Printf("[ALAT] Step 1c succeeded for %s with trackingID: %s", normalizedPhone, trackingID)

			// 4. Poll GetPartnershipAccountDetails with a bounded backoff loop
			const (
				maxAccountPollAttempts = 4
				accountPollInterval    = 1000 * time.Millisecond
			)

			for attempt := 1; attempt <= maxAccountPollAttempts; attempt++ {
				select {
				case <-providerCtx.Done():
					return nil, fmt.Errorf("context canceled while waiting for account creation: %w", providerCtx.Err())
				case <-time.After(accountPollInterval):
				}

				details, fetchErr := s.alat.GetPartnershipAccountDetails(providerCtx, normalizedPhone)
				if fetchErr == nil && details != nil && strings.TrimSpace(details.AccountNumber) != "" {
					acctDetails = details
					log.Printf("[ALAT] Account creation resolved on attempt %d for phone %s (account: %s)", attempt, normalizedPhone, details.AccountNumber)
					break
				}
			}
		}

		if acctDetails == nil || strings.TrimSpace(acctDetails.AccountNumber) == "" {
			return nil, fmt.Errorf("wema alat account was not generated yet, please try again shortly")
		}

		// Lift PND restriction with retry so the account can be debited
		var pndErr error
		const maxPndAttempts = 3
		for attempt := 1; attempt <= maxPndAttempts; attempt++ {
			pndErr = s.alat.PartnerDebitRestrictionManagement(providerCtx, "LiftPnd", acctDetails.AccountNumber)
			if pndErr == nil {
				log.Printf("[ALAT] Successfully lifted PND debit restriction on account %s (attempt %d)", acctDetails.AccountNumber, attempt)
				break
			}
			log.Printf("[ALAT] Attempt %d/%d to lift PND on account %s failed: %v", attempt, maxPndAttempts, acctDetails.AccountNumber, pndErr)
			if attempt < maxPndAttempts {
				select {
				case <-providerCtx.Done():
					return nil, providerCtx.Err()
				case <-time.After(time.Duration(attempt*500) * time.Millisecond):
				}
			}
		}
		if pndErr != nil {
			log.Printf("[ALAT] CRITICAL: failed to lift debit restriction (PND) on account %s after retries: %v", acctDetails.AccountNumber, pndErr)
			return nil, fmt.Errorf("failed to lift debit restriction on account: %w", pndErr)
		}

		fullName := strings.TrimSpace(acctDetails.FirstName + " " + acctDetails.LastName)
		if fullName == "" {
			fullName = strings.TrimSpace(req.FirstName + " " + req.LastName)
		}

		walletResp = &auth.WalletResponse{
			Customer: &auth.WalletCustomer{
				ID:          mobileUserID,
				PhoneNumber: normalizedPhone,
				FirstName:   req.FirstName,
				LastName:    req.LastName,
				Email:       normalizedEmail,
				DateOfBirth: req.Dob,
				Currency:    "NGN",
				Tier:        "2",
			},
			Wallet: &auth.WalletInfo{
				AccountNumber: acctDetails.AccountNumber,
				AccountName:   fullName,
				BankCode:      "000017",
				BankName:      "WEMA BANK",
				WalletId:      acctDetails.AccountNumber,
				Status:        "active",
			},
		}
	} else {
		if walletGenerator == nil {
			return nil, fmt.Errorf("wallet generation service is not configured")
		}
		walletResp, err = walletGenerator.GenerateWallet(providerCtx, &auth.WalletPayload{
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
			return nil, err
		}
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
		IsNinVerified:          false,
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

		deviceService := device.NewService(*deviceRepo, s.auditLogger)
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

		referralsService := referrals.NewService(referrals.NewRepository(txDB), s.auditLogger)

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

	if strings.EqualFold(strings.TrimSpace(provider), "wema") || strings.EqualFold(strings.TrimSpace(provider), "alat") {
		if s.alat == nil {
			return "", "", false, fmt.Errorf("wema alat validation service is not configured")
		}
		// If sessionIssuer is configured, validate BVN via the active KYC provider (Tendar or Prembly)
		// without requiring SMS OTP, since Face Biometrics will be used instead.
		if s.sessionIssuer != nil {
			bvnResp, err := s.sessionIssuer.ValidateBVN(ctx, strings.TrimSpace(request.Bvn))
			if err != nil {
				if recordErr := s.createFailedIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, "wema", err); recordErr != nil {
					log.Printf("ValidateBVN (ALAT): failed to record failed verification: %v", recordErr)
				}
				return "", "", false, err
			}
			return bvnResp.GetVerificationID(), "", false, nil
		}

		phone := strings.TrimSpace(request.PhoneNumber)
		if phone == "" {
			return "", "", false, fmt.Errorf("phone_number is required for wema alat bvn validation")
		}
		normalizedPhone, err := phoneutil.NormalizeNigerianNumber(phone)
		if err != nil {
			return "", "", false, err
		}
		trackingID, err := s.alat.PostPartnershipAccountCreationWithBvn(ctx, normalizedPhone, strings.TrimSpace(request.Email), strings.TrimSpace(request.Bvn))
		if err != nil {
			if recordErr := s.createFailedIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, "wema", err); recordErr != nil {
				log.Printf("ValidateBVN (ALAT): failed to record failed verification: %v", recordErr)
			}
			return "", "", false, err
		}
		verificationID, err = s.createPendingIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, trackingID, "wema")
		if err != nil {
			return "", "", false, err
		}
		return verificationID, trackingID, true, nil
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
		if recordErr := s.createFailedIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, "optimus", err); recordErr != nil {
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
	verificationID, err = s.createPendingIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, providerReferenceID, "optimus")
	if err != nil {
		return "", "", false, err
	}
	return verificationID, providerReferenceID, true, nil
}

func (s *Service) ValidateNINWithOptimus(ctx context.Context, request OptimusNINValidationRequest) (string, string, error) {
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
	verificationID, err := s.createPendingIdentityVerification(ctx, models.VerificationTypeNIN, request.Nin, providerReferenceID, "optimus")
	if err != nil {
		return "", "", err
	}
	return verificationID, providerReferenceID, nil
}

// ValidateBVNWithFace performs face biometric verification against the BVN on record using
// the active KYC provider (Tendar or Prembly) configured in system preferences.
func (s *Service) ValidateBVNWithFace(ctx context.Context, req BVNWithFaceRequest) (string, error) {
	if s.sessionIssuer == nil {
		return "", fmt.Errorf("session issuer is not configured")
	}

	info, err := s.sessionIssuer.ValidateBVNWithFace(ctx, auth.BVNWithFaceValidationRequest{
		VerificationID: req.VerificationID,
		BVN:            req.BVN,
		Image:          req.Image,
	})
	if err != nil {
		return "", err
	}

	// Update verification record to verified so Register recognizes it
	if req.VerificationID != "" && s.repo != nil {
		if err := s.repo.MarkVerificationVerified(ctx, req.VerificationID, time.Now().UTC()); err != nil {
			return "", fmt.Errorf("failed to mark verification record verified: %w", err)
		}
	}

	return info.FaceCheckID, nil
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
func (s *Service) VerifyOTP(ctx context.Context, phone, otpToken, email, referenceID string) error {
	var provider string
	if s.repo != nil {
		rec, err := s.repo.GetVerificationByProviderReference(ctx, referenceID)
		if err == nil && rec != nil {
			provider = strings.TrimSpace(rec.Provider)
		}
	}
	if provider == "" {
		p, err := s.CurrentProvider(ctx)
		if err != nil {
			return err
		}
		provider = p
	}

	if strings.EqualFold(provider, "wema") || strings.EqualFold(provider, "alat") {
		if s.alat == nil {
			return fmt.Errorf("wema alat validation service is not configured")
		}
		normalizedPhone, err := phoneutil.NormalizeNigerianNumber(phone)
		if err != nil {
			return err
		}
		err = s.alat.ValidateBVNandEnqueueAccountCreation(ctx, normalizedPhone, strings.TrimSpace(otpToken), strings.TrimSpace(referenceID))
		if err != nil {
			return err
		}
		if s.repo != nil {
			return s.repo.MarkVerificationVerifiedByProviderReference(ctx, strings.TrimSpace(referenceID), time.Now().UTC())
		}
		return nil
	}

	if s.optimus == nil {
		return fmt.Errorf("optimus validation service is not configured")
	}
	if err := s.optimus.VerifyOTPWithOptimus(ctx, phone, otpToken, email, referenceID); err != nil {
		return err
	}
	if s.repo != nil {
		return s.repo.MarkVerificationVerifiedByProviderReference(ctx, strings.TrimSpace(referenceID), time.Now().UTC())
	}
	return nil
}

// ResendOTP asks Optimus or ALAT to resend the OTP tied to referenceID, returning the
// new reference id the caller must use for the next verify/resend. Also
// repoints the pending verification record at the new reference id.
func (s *Service) ResendOTP(ctx context.Context, referenceID string) (string, error) {
	var provider string
	if s.repo != nil {
		rec, err := s.repo.GetVerificationByProviderReference(ctx, referenceID)
		if err == nil && rec != nil {
			provider = strings.TrimSpace(rec.Provider)
		}
	}
	if provider == "" {
		p, err := s.CurrentProvider(ctx)
		if err != nil {
			return "", err
		}
		provider = p
	}

	if strings.EqualFold(provider, "wema") || strings.EqualFold(provider, "alat") {
		if s.alat == nil {
			return "", fmt.Errorf("wema alat validation service is not configured")
		}
		if err := s.alat.ResendOtp(ctx, strings.TrimSpace(referenceID)); err != nil {
			return "", err
		}
		return referenceID, nil
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
