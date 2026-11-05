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
func (s *Service) Register(ctx context.Context, req OptimusRegisterRequest, ip string) (_ *RegisterResponse, retErr error) {
	rl := newOpLogger("register")
	defer rl.finish(&retErr)

	rl.step("input")
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
		return nil, fmt.Errorf("phone number is not verified")
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
		return nil, fmt.Errorf("email is not verified")
	}

	// NIN is collected and persisted below but no longer gates registration -
	// it isn't sent to either provider's wallet-creation call, so validating
	// it up front only added friction without changing the outcome.

	// A local user for this phone means registration already completed here
	// previously - fail fast with a clear error rather than calling the
	// wallet provider again, which would just be rejected as a duplicate.
	rl.step("duplicate_check")
	authRepo := auth.NewRespository(s.repo.db, s.repo.cipher)
	if existing, lookupErr := authRepo.GetUserByPhone(ctx, normalizedPhone); lookupErr == nil {
		rl.Warnf("user already exists for this phone (user_id=%s)", existing.ID)
		return nil, appErr.ErrUserExists
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return nil, lookupErr
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
			rl.Warnf("optimus rejected this bvn earlier (%s) - falling back to providus", describeBVNRecord(latestBVNRecord))

		default:
			// Pending: validate-bvn succeeded but the OTP challenge was never
			// confirmed (or failed). This is not a fallback case - falling back
			// here would let someone route around the OTP consent step just by
			// not completing it, so the user must resolve it instead.
			return nil, fmt.Errorf("bvn otp verification is not complete")
		}

	case "wema", "alat":
		if s.alat == nil {
			rl.Errorf("alat client is nil - SetAlatValidator was not called")
			return nil, fmt.Errorf("wema alat service is not configured")
		}

		switch {
		case latestBVNRecord == nil:
			return nil, fmt.Errorf("bvn is not verified")
		case latestBVNRecord.Status == models.VerificationStatusVerified:
			// Expired/used records were already rejected by checkBVNRecordUsable.
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
	rl.with("actual_provider", actualProvider)

	rl.step("input")
	dob, err := time.Parse("2006-01-02", strings.TrimSpace(req.Dob))
	if err != nil {
		return nil, fmt.Errorf("invalid date of birth: %w", err)
	}

	mobileUserID := uuid.NewString()
	internalWalletID := uuid.NewString()
	rl.with("user_id", mobileUserID)

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

		// 1. Resolve and verify the face check record. Required on every path,
		// including reuse of an existing account below - otherwise anyone with
		// verified phone/email/BVN records could claim an existing Wema account
		// without ever passing the face check.
		// Security: live image MUST come from an authoritative, matched
		// FaceCheckRecord linked to this BVN. Client-supplied raw images or
		// unverified/mismatched face checks are rejected.
		rl.step("face_check")
		faceRecord, err := s.resolveMatchedFaceRecord(ctx, requestedFace, bvnRecord)
		if err != nil {
			return nil, err
		}
		if faceRecord == nil || faceRecord.FaceImageProvided == nil || strings.TrimSpace(*faceRecord.FaceImageProvided) == "" {
			rl.Warnf("no usable face check: %s requested_face=%s resolved_face=%s",
				describeBVNRecord(bvnRecord), describeFaceRecord(requestedFace), describeFaceRecord(faceRecord))
			return nil, fmt.Errorf("face biometric verification is required for wema registration and must be verified")
		}
		rl.with("face_check", faceRecord.ID)

		rl.step("alat_account_lookup")

		// 2. Check if the account already exists on Wema (e.g. from an earlier
		// attempt whose DB write failed/timed out). Reusing the existing account
		// prevents ALAT duplicate errors on retry. Only a clean "no account"
		// answer (success with empty data, or ALAT's own error envelope) may
		// lead to creation - a network/timeout error says nothing about whether
		// an account exists, so creating one then could duplicate it.
		existingDetails, fetchErr := s.alat.GetPartnershipAccountDetails(providerCtx, normalizedPhone)
		if fetchErr != nil {
			var alatErr *appErr.AlatError
			if !errors.As(fetchErr, &alatErr) {
				rl.Errorf("transient alat account lookup error, not creating an account: %v", fetchErr)
				return nil, fmt.Errorf("wema is temporarily unavailable, please try again shortly")
			}
			rl.Infof("alat reports no account yet (%s) - will create", describeAlatError(alatErr))
		} else if existingDetails == nil || strings.TrimSpace(existingDetails.AccountNumber) == "" {
			rl.Infof("alat returned no account number - will create")
		}

		if fetchErr == nil && existingDetails != nil && strings.TrimSpace(existingDetails.AccountNumber) != "" {
			if !alatAccountMatchesApplicant(existingDetails, normalizedEmail, req.FirstName, req.LastName) {
				rl.Criticalf("existing alat account %s does not match applicant, refusing reuse: %s",
					existingDetails.AccountNumber, describeIdentityMismatch(existingDetails, normalizedEmail, req.FirstName, req.LastName))
				return nil, fmt.Errorf("a wema account already exists for this phone under different details")
			}
			acctDetails = existingDetails
			rl.with("account", existingDetails.AccountNumber)
			rl.Infof("reusing existing alat account, skipping Tier 2 creation")
		} else {
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

			rl.step("alat_tier2_create")
			rl.with("correlation_id", tier2Req.CorrelationID)
			trackingID, tier2Err := s.alat.PostTier2PartnershipAccountWithFace(providerCtx, tier2Req)
			if tier2Err != nil {
				return nil, fmt.Errorf("wema alat tier 2 account creation failed: %w", tier2Err)
			}
			rl.with("tracking_id", trackingID)
			rl.Infof("tier 2 account creation accepted by alat")

			rl.step("alat_account_poll")

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
					rl.with("account", details.AccountNumber)
					rl.Infof("alat account available on poll attempt %d/%d", attempt, maxAccountPollAttempts)
					break
				}
				rl.Infof("alat account not ready on poll attempt %d/%d (err=%v)", attempt, maxAccountPollAttempts, fetchErr)
			}
		}

		if acctDetails == nil || strings.TrimSpace(acctDetails.AccountNumber) == "" {
			rl.Warnf("alat accepted tier 2 creation but no account appeared while polling - a retry should pick it up via the existing-account path")
			return nil, fmt.Errorf("wema alat account was not generated yet, please try again shortly")
		}

		rl.step("alat_lift_pnd")

		// Lift PND restriction with retry so the account can be debited
		var pndErr error
		const maxPndAttempts = 3
		for attempt := 1; attempt <= maxPndAttempts; attempt++ {
			pndErr = s.alat.PartnerDebitRestrictionManagement(providerCtx, "LiftPnd", acctDetails.AccountNumber)
			if pndErr == nil {
				rl.Infof("lifted PND on attempt %d/%d", attempt, maxPndAttempts)
				break
			}
			rl.Warnf("lift PND attempt %d/%d failed: %v", attempt, maxPndAttempts, pndErr)
			if attempt < maxPndAttempts {
				select {
				case <-providerCtx.Done():
					return nil, providerCtx.Err()
				case <-time.After(time.Duration(attempt*500) * time.Millisecond):
				}
			}
		}
		if pndErr != nil {
			rl.Criticalf("could not lift PND after %d attempts - account exists at alat but nothing is saved locally yet: %v", maxPndAttempts, pndErr)
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
		rl.step("wallet_generate")
		if walletGenerator == nil {
			rl.Errorf("wallet generator for %s is nil - check wiring in router.go", actualProvider)
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
		rl.Criticalf("wallet provider returned an incomplete response (customer_nil=%t wallet_nil=%t) - an account may exist at the provider",
			walletResp == nil || walletResp.Customer == nil, walletResp == nil || walletResp.Wallet == nil)
		return nil, fmt.Errorf("wallet provider returned an incomplete response")
	}
	rl.with("account", walletResp.Wallet.AccountNumber)

	rl.step("persist")

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
		// The provider account exists but the user/wallet rows were rolled
		// back - this needs reconciling by hand if the user doesn't retry.
		rl.Criticalf("provider account created but saving user/wallet failed (rolled back) - provider=%s account=%s: %v",
			actualProvider, walletResp.Wallet.AccountNumber, err)
		return nil, err
	}

	rl.step("session")
	if s.sessionIssuer == nil {
		rl.Errorf("session issuer is nil - user %s was created but cannot log in until it is wired", mobileUserID)
		return nil, fmt.Errorf("session issuer is not configured")
	}
	session, err := s.sessionIssuer.IssueSessionTokens(providerCtx, mobileUserID, req.Device.DeviceID, ip)
	if err != nil {
		rl.Warnf("user created but session issuance failed - the user can log in normally: %v", err)
		return nil, err
	}
	rl.step("")

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
		// If sessionIssuer is configured, validate BVN via the active KYC provider (Tendar or Prembly)
		// without requiring SMS OTP, since Face Biometrics will be used instead.
		if s.sessionIssuer != nil {
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

		phone := strings.TrimSpace(request.PhoneNumber)
		if phone == "" {
			return "", "", false, fmt.Errorf("phone_number is required for wema alat bvn validation")
		}
		normalizedPhone, err := phoneutil.NormalizeNigerianNumber(phone)
		if err != nil {
			return "", "", false, err
		}
		rl.step("alat_bvn_otp")
		trackingID, err := s.alat.PostPartnershipAccountCreationWithBvn(ctx, normalizedPhone, strings.TrimSpace(request.Email), strings.TrimSpace(request.Bvn))
		if err != nil {
			if recordErr := s.createFailedIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, "wema", err); recordErr != nil {
				rl.Errorf("could not save the failed verification record: %v", recordErr)
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
			return "", fmt.Errorf("bvn verification record not found")
		}
		rl.Infof("loaded %s", describeBVNRecord(rec))
		if rec.SubjectHash != bvnSubjectHash(req.BVN) {
			rl.Warnf("request bvn does not hash to the record's subject_hash - client sent a different bvn than it validated")
			return "", fmt.Errorf("bvn does not match the verification record")
		}
		switch rec.Status {
		case models.VerificationStatusVerified, models.VerificationStatusPending:
		case models.VerificationStatusUsed:
			return "", fmt.Errorf("this bvn has already been used for registration")
		default:
			return "", fmt.Errorf("bvn verification failed, please validate again")
		}
		if isExpired(rec.ExpiresAt, time.Now().UTC()) {
			return "", fmt.Errorf("bvn verification has expired, please validate again")
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
		return fmt.Errorf("this bvn has already been used for registration")
	case rec.Status == models.VerificationStatusVerified && isExpired(rec.ExpiresAt, time.Now().UTC()):
		return fmt.Errorf("bvn verification has expired, please validate again")
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

// alatAccountMatchesApplicant reports whether an existing ALAT account looks
// like it belongs to this applicant. Each field ALAT returns must match;
// fields ALAT leaves empty are not held against the applicant.
func alatAccountMatchesApplicant(d *AlatAccountDetails, email, firstName, lastName string) bool {
	if d == nil {
		return false
	}
	if v := strings.TrimSpace(d.Email); v != "" && !strings.EqualFold(v, strings.TrimSpace(email)) {
		return false
	}
	if v := normalizeName(d.FirstName); v != "" && v != normalizeName(firstName) {
		return false
	}
	if v := normalizeName(d.LastName); v != "" && v != normalizeName(lastName) {
		return false
	}
	return true
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
			rl.step("mark_verified")
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
