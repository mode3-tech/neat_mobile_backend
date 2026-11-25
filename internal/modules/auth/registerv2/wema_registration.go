package registerv2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/logredact"
	"neat_mobile_app_backend/internal/modules/auth"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/modules/referrals"
	"neat_mobile_app_backend/internal/modules/wallet"
	phoneutil "neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Registration outcomes reported to the client.
const (
	RegistrationStatusCompleted = "completed"
	RegistrationStatusPending   = "pending"
	RegistrationStatusFailed    = "failed"
	RegistrationStatusExpired   = "expired"
)

const (
	// pendingWemaTTL bounds how long a pending registration waits for its
	// ALAT account before it's expired (and flagged for manual follow-up).
	pendingWemaTTL = auth.BVNVerificationTTL
	// pendingWemaPollRetryAfter stops a client polling the status endpoint
	// from hitting ALAT on every request.
	pendingWemaPollRetryAfter = 10 * time.Second
	// pendingWemaSweepRetryAfter spaces the sweep's attempts per row.
	pendingWemaSweepRetryAfter = time.Minute
	// pendingWemaStuckAfter returns a row left processing by a crashed worker
	// to pending.
	pendingWemaStuckAfter = 5 * time.Minute
)

var (
	// errRegistrationNotFound covers an unknown ID and a wrong claim token
	// alike, so the status endpoint doesn't reveal which IDs exist.
	errRegistrationNotFound error = &userError{msg: "registration not found"}
	errRegistrationClaimed  error = &userError{msg: "registration is complete and its session was already issued - please log in"}
	errWemaAccountNotReady  error = &userError{msg: "your wema account is still being opened, please try again shortly"}
)

// registration is everything needed to save a user once the provider account
// exists - built from a /register request, or rebuilt from a pending row.
type registration struct {
	mobileUserID      string
	internalWalletID  string
	phone             string
	email             string
	firstName         string
	lastName          string
	dobRaw            string
	dob               time.Time
	address           string
	bvn               string
	nin               string
	passwordHash      string
	pinHash           string
	biometrics        bool
	referralCode      string
	device            DeviceRegisteration
	ip                string
	phoneVerification string
	emailVerification string
	bvnVerification   string
}

// registerWema opens (or reuses) the customer's ALAT tier 2 account and
// finishes the registration. If ALAT is still queueing the account when the
// request must answer, and the client supports it, the registration is saved
// as pending and completed later by the Account Creation webhook, the sweep,
// or the client's status poll.
func (s *Service) registerWema(ctx context.Context, rl *opLogger, req OptimusRegisterRequest, reg *registration, bvnRecord *models.VerificationRecord) (*RegisterResponse, error) {
	// 1. Wema's own face biometric check must have been done for this BVN or
	// NIN: its web app reports a correlationId to our facial-verification
	// callback, and ALAT only opens a tier 2 account against one of those.
	// Required on every path, including reuse of an existing account below -
	// otherwise anyone with verified phone/email/BVN records could claim an
	// existing Wema account without ever passing the face check.
	rl.step("face_check")
	faceCorrelationID, err := s.wemaFaceCorrelation(ctx, rl, reg.bvn, reg.nin)
	if err != nil {
		rl.Warnf("cannot continue without wema's face verification (the face web app's callback may not have arrived - check the [ALAT webhook] event=facial_verification logs and the IP allowlist): %v", err)
		return nil, err
	}
	rl.with("face_correlation_id", faceCorrelationID)

	// 2. An earlier attempt is already waiting on its ALAT account: resume it
	// with this request's details instead of asking ALAT again.
	rl.step("pending_lookup")
	if s.repo != nil && s.repo.db != nil {
		existing, err := s.repo.GetActivePendingWemaRegistrationByPhone(ctx, reg.phone)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return s.resumePendingWema(ctx, rl, req, reg, existing)
		}
	}

	// 3. Check if the account already exists on Wema (e.g. from an earlier
	// attempt whose DB write failed/timed out). Reusing the existing account
	// prevents ALAT duplicate errors on retry. Only a clean "no account"
	// answer (success with empty data, or ALAT's own error envelope) may
	// lead to creation - a network/timeout error says nothing about whether
	// an account exists, so creating one then could duplicate it.
	rl.step("alat_account_lookup")
	existingDetails, fetchErr := s.alat.GetPartnershipAccountDetails(ctx, reg.phone)
	if fetchErr != nil {
		var alatErr *appErr.AlatError
		if !errors.As(fetchErr, &alatErr) || alatErr.NoResponse {
			rl.Errorf("transient alat account lookup error, not creating an account: %v", fetchErr)
			return nil, userErrorf("wema is temporarily unavailable, please try again shortly")
		}
		rl.Infof("alat reports no account yet (%s) - will create", describeAlatError(alatErr))
	} else if existingDetails == nil || strings.TrimSpace(existingDetails.AccountNumber) == "" {
		rl.Infof("alat returned no account number - will create")
	}

	if fetchErr == nil && existingDetails != nil && strings.TrimSpace(existingDetails.AccountNumber) != "" {
		if !alatAccountMatchesApplicant(existingDetails, bvnRecord, reg.firstName, reg.lastName) {
			rl.Criticalf("existing alat account %s does not match applicant, refusing reuse: %s",
				existingDetails.AccountNumber, describeIdentityMismatch(existingDetails, reg.email, reg.firstName, reg.lastName))
			return nil, userErrorf("a wema account already exists for this phone under different details")
		}
		rl.with("account", existingDetails.AccountNumber)
		rl.Infof("reusing existing alat account, skipping Tier 2 creation")
		return s.finishWema(ctx, rl, reg, existingDetails, nil)
	}

	// 4. Invoke Step 1c (Tier 2 Face Account Creation) without residential address
	tier2Req := AlatTier2FaceRequest{
		BVN:                reg.bvn,
		NIN:                reg.nin,
		PhoneNumber:        reg.phone,
		EmailAddress:       reg.email,
		ResidentialAddress: nil, // Omitted for Tier 2; address is added when upgrading to Tier 3
		CorrelationID:      faceCorrelationID,
	}
	rl.step("alat_tier2_create")
	rl.with("correlation_id", tier2Req.CorrelationID)
	trackingID, err := s.alat.PostTier2PartnershipAccountWithFace(ctx, tier2Req)
	pendingMsg := ""
	switch {
	case errors.Is(err, ErrAlatAccountGenerationPending):
		var pe *AlatPendingError
		if errors.As(err, &pe) {
			pendingMsg = pe.Message
		}
		// An earlier request is still being generated at ALAT; wait for it
		// exactly as if this one had just been accepted.
		rl.with("tracking_id", trackingID)
		rl.Infof("alat is already generating an account for these details - waiting for it")
	case err != nil:
		// Wema's own words: a rejected c_id (unknown, already used, or not
		// matching this BVN/NIN) shows up here and in the [ALAT] POST line
		// above, which has the HTTP status and raw body.
		var alatErr *appErr.AlatError
		if errors.As(err, &alatErr) {
			rl.Errorf("alat rejected tier 2 creation for c_id=%s: %s no_response=%t", tier2Req.CorrelationID, describeAlatError(alatErr), alatErr.NoResponse)
		} else {
			rl.Errorf("tier 2 creation for c_id=%s failed: %v", tier2Req.CorrelationID, err)
		}
		return nil, fmt.Errorf("wema alat tier 2 account creation failed: %w", err)
	default:
		rl.with("tracking_id", trackingID)
		rl.Infof("tier 2 account creation accepted by alat")
	}

	// 5. Poll briefly - ALAT usually finishes quickly, and finishing inline
	// saves the client a round trip.
	rl.step("alat_account_poll")
	const maxAccountPollAttempts = 4
	accountPollInterval := time.Second
	if s.wemaPollInterval > 0 {
		accountPollInterval = s.wemaPollInterval
	}
	for attempt := 1; attempt <= maxAccountPollAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled while waiting for account creation: %w", ctx.Err())
		case <-time.After(accountPollInterval):
		}
		details, err := s.alat.GetPartnershipAccountDetails(ctx, reg.phone)
		if err == nil && details != nil && strings.TrimSpace(details.AccountNumber) != "" {
			rl.with("account", details.AccountNumber)
			rl.Infof("alat account available on poll attempt %d/%d", attempt, maxAccountPollAttempts)
			return s.finishWema(ctx, rl, reg, details, nil)
		}
		rl.Infof("alat account not ready on poll attempt %d/%d (err=%v)", attempt, maxAccountPollAttempts, err)
	}

	// 6. Still queued at ALAT. Hand off to the pending flow if the client can
	// follow it; older clients get "try again" and pick the account up on
	// retry through the existing-account path above.
	if !req.SupportsPending {
		rl.Warnf("alat accepted tier 2 creation but no account appeared while polling - a retry should pick it up via the existing-account path")
		return nil, errWemaAccountNotReady
	}
	rl.step("pending_create")
	pending, claimToken, err := s.newPendingWema(reg, trackingID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreatePendingWemaRegistration(ctx, pending); err != nil {
		return nil, err
	}
	rl.with("registration_id", pending.ID)
	rl.Infof("alat account still queued - saved pending registration for the webhook/sweep to complete")
	resp := pendingResponse(pending.ID, claimToken)
	resp.Message = pendingMsg
	return resp, nil
}

// resumePendingWema handles a /register for a phone whose earlier attempt is
// still pending: the newest request's details replace the old ones (the
// customer may have changed their password, device...), the claim token is
// rotated so this client can follow it, and completion is tried right away.
func (s *Service) resumePendingWema(ctx context.Context, rl *opLogger, req OptimusRegisterRequest, reg *registration, existing *PendingWemaRegistration) (*RegisterResponse, error) {
	rl.with("registration_id", existing.ID)
	rl.step("pending_resume")

	// The ALAT account was requested with the pending row's BVN and NIN; a
	// retry with different ones can't take it over.
	prev, err := s.registrationFromPending(existing)
	if err != nil {
		return nil, err
	}
	if prev.bvn != reg.bvn || prev.nin != reg.nin {
		rl.Warnf("retry uses a different bvn/nin than the pending registration - refusing")
		return nil, userErrorf("a registration with different BVN or NIN details is already in progress for this phone")
	}

	refreshed, claimToken, err := s.newPendingWema(reg, existing.TrackingID)
	if err != nil {
		return nil, err
	}
	refreshed.ID = existing.ID
	updated, err := s.repo.RefreshPendingWemaRegistration(ctx, refreshed)
	if err != nil {
		return nil, err
	}
	if !updated {
		// A worker is completing it right now; keep its data, but still give
		// this client a claim token it can follow.
		if err := s.repo.RotatePendingWemaClaimToken(ctx, existing.ID, refreshed.ClaimTokenHash); err != nil {
			return nil, err
		}
		rl.Infof("pending registration is being completed by another worker - returning it as pending")
		return s.pendingOrRetry(req, existing.ID, claimToken)
	}

	completed, err := s.tryCompletePendingWema(ctx, existing.ID)
	if err != nil {
		return nil, err
	}
	if !completed {
		rl.Infof("pending registration still waiting on alat")
		return s.pendingOrRetry(req, existing.ID, claimToken)
	}
	// Completed inline: hand the session to this request directly.
	if _, err := s.repo.ClaimPendingWemaRegistration(ctx, existing.ID, time.Now().UTC()); err != nil {
		return nil, err
	}
	p, err := s.repo.GetPendingWemaRegistration(ctx, existing.ID)
	if err != nil || p == nil {
		return nil, fmt.Errorf("pending registration disappeared after completion: %v", err)
	}
	return s.issueSession(ctx, rl, p.MobileUserID, p.DeviceID, reg.ip)
}

func (s *Service) pendingOrRetry(req OptimusRegisterRequest, id, claimToken string) (*RegisterResponse, error) {
	if !req.SupportsPending {
		return nil, errWemaAccountNotReady
	}
	return pendingResponse(id, claimToken), nil
}

func pendingResponse(id, claimToken string) *RegisterResponse {
	return &RegisterResponse{
		Status:         RegistrationStatusPending,
		RegistrationID: id,
		ClaimToken:     claimToken,
	}
}

// newPendingWema builds a pending row from reg and a fresh claim token. The
// token is returned in clear once; only its hash is stored.
func (s *Service) newPendingWema(reg *registration, trackingID string) (*PendingWemaRegistration, string, error) {
	encBVN, err := s.repo.Encrypt(reg.bvn)
	if err != nil {
		return nil, "", err
	}
	encNIN, err := s.repo.Encrypt(reg.nin)
	if err != nil {
		return nil, "", err
	}
	token, err := newClaimToken()
	if err != nil {
		return nil, "", err
	}
	return &PendingWemaRegistration{
		ID:                uuid.NewString(),
		MobileUserID:      reg.mobileUserID,
		InternalWalletID:  reg.internalWalletID,
		Phone:             reg.phone,
		Email:             reg.email,
		FirstName:         reg.firstName,
		LastName:          reg.lastName,
		DOB:               reg.dobRaw,
		Address:           reg.address,
		BVNEncrypted:      encBVN,
		NINEncrypted:      encNIN,
		PasswordHash:      reg.passwordHash,
		PinHash:           reg.pinHash,
		Biometrics:        reg.biometrics,
		ReferralCode:      reg.referralCode,
		DeviceID:          reg.device.DeviceID,
		DevicePublicKey:   reg.device.PublicKey,
		DeviceName:        reg.device.DeviceName,
		DeviceModel:       reg.device.DeviceModel,
		DeviceOS:          reg.device.OS,
		DeviceOSVersion:   reg.device.OSVersion,
		DeviceAppVersion:  reg.device.AppVersion,
		IP:                reg.ip,
		PhoneVerification: reg.phoneVerification,
		EmailVerification: reg.emailVerification,
		BVNVerification:   reg.bvnVerification,
		TrackingID:        trackingID,
		ClaimTokenHash:    hashClaimToken(token),
		Status:            PendingWemaStatusPending,
		ExpiresAt:         time.Now().UTC().Add(pendingWemaTTL),
	}, token, nil
}

// registrationFromPending rebuilds the registration a pending row was saved
// from.
func (s *Service) registrationFromPending(p *PendingWemaRegistration) (*registration, error) {
	bvn, err := s.repo.Decrypt(p.BVNEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt pending bvn: %w", err)
	}
	nin, err := s.repo.Decrypt(p.NINEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt pending nin: %w", err)
	}
	dob, err := time.Parse("2006-01-02", p.DOB)
	if err != nil {
		return nil, fmt.Errorf("pending registration has an invalid dob: %w", err)
	}
	return &registration{
		mobileUserID:     p.MobileUserID,
		internalWalletID: p.InternalWalletID,
		phone:            p.Phone,
		email:            p.Email,
		firstName:        p.FirstName,
		lastName:         p.LastName,
		dobRaw:           p.DOB,
		dob:              dob,
		address:          p.Address,
		bvn:              bvn,
		nin:              nin,
		passwordHash:     p.PasswordHash,
		pinHash:          p.PinHash,
		biometrics:       p.Biometrics,
		referralCode:     p.ReferralCode,
		device: DeviceRegisteration{
			DeviceID:    p.DeviceID,
			PublicKey:   p.DevicePublicKey,
			DeviceName:  p.DeviceName,
			DeviceModel: p.DeviceModel,
			OS:          p.DeviceOS,
			OSVersion:   p.DeviceOSVersion,
			AppVersion:  p.DeviceAppVersion,
		},
		ip:                p.IP,
		phoneVerification: p.PhoneVerification,
		emailVerification: p.EmailVerification,
		bvnVerification:   p.BVNVerification,
	}, nil
}

// finishWema lifts PND on the ALAT account, saves the user and wallet, and
// issues the session - the synchronous end of a /register call.
func (s *Service) finishWema(ctx context.Context, rl *opLogger, reg *registration, acct *AlatAccountDetails, inTx func(*gorm.DB) error) (*RegisterResponse, error) {
	if err := s.liftPndWithRetry(ctx, rl, acct.AccountNumber); err != nil {
		return nil, err
	}
	if err := s.persistRegistration(ctx, rl, reg, wemaWalletResponse(reg, acct), "wema", inTx); err != nil {
		return nil, err
	}
	return s.issueSession(ctx, rl, reg.mobileUserID, reg.device.DeviceID, reg.ip)
}

// liftPndWithRetry lifts the post-no-debit restriction ALAT places on new
// accounts so they can be debited.
func (s *Service) liftPndWithRetry(ctx context.Context, rl *opLogger, accountNumber string) error {
	rl.step("alat_lift_pnd")
	const maxPndAttempts = 3
	var pndErr error
	for attempt := 1; attempt <= maxPndAttempts; attempt++ {
		pndErr = s.alat.PartnerDebitRestrictionManagement(ctx, "LiftPnd", accountNumber)
		if pndErr == nil {
			rl.Infof("lifted PND on attempt %d/%d", attempt, maxPndAttempts)
			return nil
		}
		rl.Warnf("lift PND attempt %d/%d failed: %v", attempt, maxPndAttempts, pndErr)
		if attempt < maxPndAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt*500) * time.Millisecond):
			}
		}
	}
	rl.Criticalf("could not lift PND after %d attempts - account exists at alat but nothing is saved locally yet: %v", maxPndAttempts, pndErr)
	return fmt.Errorf("failed to lift debit restriction on account: %w", pndErr)
}

func wemaWalletResponse(reg *registration, acct *AlatAccountDetails) *auth.WalletResponse {
	fullName := strings.TrimSpace(acct.FirstName + " " + acct.LastName)
	if fullName == "" {
		fullName = strings.TrimSpace(reg.firstName + " " + reg.lastName)
	}
	return &auth.WalletResponse{
		Customer: &auth.WalletCustomer{
			ID:          reg.mobileUserID,
			PhoneNumber: reg.phone,
			FirstName:   reg.firstName,
			LastName:    reg.lastName,
			Email:       reg.email,
			DateOfBirth: reg.dobRaw,
			Currency:    "NGN",
			Tier:        "2",
		},
		Wallet: &auth.WalletInfo{
			AccountNumber: strings.TrimSpace(acct.AccountNumber),
			AccountName:   fullName,
			BankCode:      "000017",
			BankName:      "WEMA BANK",
			WalletId:      strings.TrimSpace(acct.AccountNumber),
			Status:        "active",
		},
	}
}

// persistRegistration saves the user, wallet, device binding, verification
// usage and referrals in one transaction. inTx, if set, runs inside the same
// transaction (the pending flow uses it to mark its row completed).
func (s *Service) persistRegistration(ctx context.Context, rl *opLogger, reg *registration, walletResp *auth.WalletResponse, provider string, inTx func(*gorm.DB) error) error {
	if s.persistOverride != nil {
		return s.persistOverride(ctx, rl, reg, walletResp, provider, inTx)
	}
	if walletResp == nil || walletResp.Customer == nil || walletResp.Wallet == nil {
		rl.Criticalf("wallet provider returned an incomplete response (customer_nil=%t wallet_nil=%t) - an account may exist at the provider",
			walletResp == nil || walletResp.Customer == nil, walletResp == nil || walletResp.Wallet == nil)
		return fmt.Errorf("wallet provider returned an incomplete response")
	}
	rl.with("account", walletResp.Wallet.AccountNumber)
	rl.step("persist")

	address := reg.address
	if walletResp.Customer.Address != nil && strings.TrimSpace(*walletResp.Customer.Address) != "" {
		address = strings.TrimSpace(*walletResp.Customer.Address)
	}

	now := time.Now().UTC()
	capExpiresAt := now.Add(24 * time.Hour)
	addressCopy := address
	emailCopy := reg.email
	user := &models.User{
		ID:              reg.mobileUserID,
		WalletID:        reg.internalWalletID,
		Phone:           reg.phone,
		Address:         &addressCopy,
		Email:           &emailCopy,
		FirstName:       reg.firstName,
		LastName:        reg.lastName,
		PasswordHash:    reg.passwordHash,
		PinHash:         reg.pinHash,
		DOB:             reg.dob,
		BVN:             reg.bvn,
		NIN:             reg.nin,
		IsEmailVerified: true,
		IsPhoneVerified: true,
		IsBvnVerified:   true,
		// ALAT validates the NIN when it opens a Wema tier 2 account, so an
		// account existing means it passed; other providers don't check it.
		IsNinVerified:          provider == "wema",
		IsBiometricsEnabled:    reg.biometrics,
		IsNotificationsEnabled: true,
		ActivationCapAmount:    s.activationCapKobo,
		ActivationCapExpiresAt: &capExpiresAt,
	}

	walletRecord := &wallet.CustomerWallet{
		ID:               uuid.NewString(),
		InternalWalletID: reg.internalWalletID,
		MobileUserID:     reg.mobileUserID,
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
		Provider:         provider,
		AccountNumber:    walletResp.Wallet.AccountNumber,
		AccountName:      walletResp.Wallet.AccountName,
		AccountRef:       walletResp.Wallet.AccountReference,
		BookedBalance:    walletResp.Wallet.BookedBalance,
		AvailableBalance: walletResp.Wallet.AvailableBalance,
		Status:           walletResp.Wallet.Status,
		WalletType:       walletResp.Wallet.WalletType,
		Updated:          walletResp.Wallet.Updated,
		CreatedAt:        now,
	}

	usedVerificationIDs := []string{reg.phoneVerification, reg.emailVerification, reg.bvnVerification}

	err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		authRepo := auth.NewRespository(txDB, s.repo.cipher)
		walletRepo := wallet.NewRepository(txDB)
		deviceRepo := device.NewRepository(txDB)
		verificationRepo := NewRepository(txDB, s.repo.cipher)

		if _, txErr := authRepo.CreateUser(ctx, user); txErr != nil {
			return txErr
		}
		if txErr := authRepo.LinkBVNRecordToUser(ctx, user.BVN, user.ID); txErr != nil {
			return txErr
		}
		if txErr := walletRepo.CreateWallet(ctx, walletRecord); txErr != nil {
			return txErr
		}

		deviceService := device.NewService(*deviceRepo, s.auditLogger)
		if txErr := deviceService.BindDevice(ctx, reg.mobileUserID, &device.DeviceBindingRequest{
			DeviceID:    reg.device.DeviceID,
			PublicKey:   reg.device.PublicKey,
			DeviceName:  reg.device.DeviceName,
			DeviceModel: reg.device.DeviceModel,
			OS:          reg.device.OS,
			OSVersion:   reg.device.OSVersion,
			AppVersion:  reg.device.AppVersion,
			IP:          reg.ip,
		}); txErr != nil {
			return txErr
		}

		for _, id := range usedVerificationIDs {
			if id == "" {
				continue
			}
			if txErr := verificationRepo.MarkVerificationUsed(ctx, id, now); txErr != nil {
				return txErr
			}
		}

		referralsService := referrals.NewService(referrals.NewRepository(txDB), s.auditLogger)
		if reg.referralCode != "" {
			if txErr := referralsService.RedeemReferralCode(ctx, reg.mobileUserID, reg.referralCode); txErr != nil {
				return txErr
			}
		}
		if txErr := referralsService.GenerateAndAssignReferralCode(ctx, reg.mobileUserID); txErr != nil {
			return txErr
		}

		if inTx != nil {
			return inTx(txDB)
		}
		return nil
	})
	if err != nil {
		// The provider account exists but the user/wallet rows were rolled
		// back - this needs reconciling by hand if nothing retries it.
		rl.Criticalf("provider account created but saving user/wallet failed (rolled back) - provider=%s account=%s: %v",
			provider, walletResp.Wallet.AccountNumber, err)
		return err
	}
	return nil
}

func (s *Service) issueSession(ctx context.Context, rl *opLogger, userID, deviceID, ip string) (*RegisterResponse, error) {
	rl.step("session")
	if s.sessionIssuer == nil {
		rl.Errorf("session issuer is nil - user %s was created but cannot log in until it is wired", userID)
		return nil, fmt.Errorf("session issuer is not configured")
	}
	session, err := s.sessionIssuer.IssueSessionTokens(ctx, userID, deviceID, ip)
	if err != nil {
		// The user and wallet are saved: registration succeeded. Retry once,
		// then report success without tokens rather than an error - an error
		// would send the client to retry /register, which now fails with
		// "user already exists".
		rl.Warnf("user created but session issuance failed, retrying once: %v", err)
		select {
		case <-ctx.Done():
		case <-time.After(500 * time.Millisecond):
			session, err = s.sessionIssuer.IssueSessionTokens(ctx, userID, deviceID, ip)
		}
	}
	if err != nil {
		rl.Warnf("user created but session issuance failed - returning success with login required: %v", err)
		rl.step("")
		return &RegisterResponse{
			Status:        RegistrationStatusCompleted,
			LoginRequired: true,
			Message:       "Your account is ready. Please log in to continue.",
		}, nil
	}
	rl.step("")
	return &RegisterResponse{
		Status:       RegistrationStatusCompleted,
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
	}, nil
}

// tryCompletePendingWema makes one attempt to finish a pending registration:
// confirm the account with ALAT (webhooks are unsigned, so their word alone
// is never trusted), check it's the applicant's, lift PND, and save the user
// and wallet. It returns true only when this call completed it. Anything
// retryable hands the row back to pending for the next webhook, sweep or poll.
func (s *Service) tryCompletePendingWema(ctx context.Context, id string) (bool, error) {
	rl := newOpLogger("complete_pending_wema")
	rl.with("registration_id", id)
	var retErr error
	defer rl.finish(&retErr)

	rl.step("claim_attempt")
	now := time.Now().UTC()
	started, err := s.repo.StartPendingWemaAttempt(ctx, id, now)
	if err != nil {
		retErr = err
		return false, err
	}
	if !started {
		rl.Infof("not pending (another worker has it, or it already finished) - skipping")
		return false, nil
	}

	release := func(reason string) (bool, error) {
		rl.Infof("will retry later: %s", reason)
		if err := s.repo.ReleasePendingWemaAttempt(ctx, id, reason); err != nil {
			retErr = err
			return false, err
		}
		return false, nil
	}
	fail := func(reason string) (bool, error) {
		rl.Criticalf("pending registration failed: %s", reason)
		if err := s.repo.FailPendingWemaRegistration(ctx, id, reason); err != nil {
			retErr = err
			return false, err
		}
		return false, nil
	}

	p, err := s.repo.GetPendingWemaRegistration(ctx, id)
	if err != nil || p == nil {
		return release(fmt.Sprintf("load pending row: %v", err))
	}
	rl.with("phone", maskPhone(p.Phone)).with("user_id", p.MobileUserID)

	rl.step("alat_account_lookup")
	acct, err := s.alat.GetPartnershipAccountDetails(ctx, p.Phone)
	if err != nil {
		return release(fmt.Sprintf("alat lookup: %v", err))
	}
	if acct == nil || strings.TrimSpace(acct.AccountNumber) == "" {
		return release("alat account not ready yet")
	}
	rl.with("account", acct.AccountNumber)

	rl.step("identity")
	bvnRecord, err := s.repo.GetVerificationByID(ctx, p.BVNVerification)
	if err != nil {
		return release(fmt.Sprintf("load bvn record: %v", err))
	}
	if !alatAccountMatchesApplicant(acct, bvnRecord, p.FirstName, p.LastName) {
		return fail(fmt.Sprintf("alat account %s does not match the applicant: %s",
			acct.AccountNumber, describeIdentityMismatch(acct, p.Email, p.FirstName, p.LastName)))
	}

	reg, err := s.registrationFromPending(p)
	if err != nil {
		return fail(err.Error())
	}
	if err := s.liftPndWithRetry(ctx, rl, acct.AccountNumber); err != nil {
		return release(err.Error())
	}
	err = s.persistRegistration(ctx, rl, reg, wemaWalletResponse(reg, acct), "wema", func(txDB *gorm.DB) error {
		return NewRepository(txDB, s.repo.cipher).CompletePendingWemaRegistration(ctx, id, acct.AccountNumber, time.Now().UTC())
	})
	if err != nil {
		// A user already holding this phone/BVN means a retry can never
		// succeed; anything else (DB hiccup) is worth retrying.
		if exists, existsErr := s.userExistsForPending(ctx, reg); existsErr == nil && exists {
			return fail("a user already exists for this phone, bvn or nin")
		}
		return release(fmt.Sprintf("persist: %v", err))
	}
	rl.Infof("pending registration completed")
	return true, nil
}

func (s *Service) userExistsForPending(ctx context.Context, reg *registration) (bool, error) {
	authRepo := auth.NewRespository(s.repo.db, s.repo.cipher)
	if _, err := authRepo.GetUserByPhone(ctx, reg.phone); err == nil {
		return true, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	return s.repo.UserExistsByIdentity(ctx, reg.bvn, reg.nin)
}

// RegistrationStatus is the client's view of a pending registration. Once
// it's completed, the first call issues the session (one time only); while
// it's pending, a call may itself try to complete it.
func (s *Service) RegistrationStatus(ctx context.Context, id, claimToken, ip string) (_ *RegisterResponse, retErr error) {
	rl := newOpLogger("registration_status")
	rl.with("registration_id", strings.TrimSpace(id))
	defer rl.finish(&retErr)

	p, err := s.repo.GetPendingWemaRegistration(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil || subtle.ConstantTimeCompare([]byte(hashClaimToken(claimToken)), []byte(p.ClaimTokenHash)) != 1 {
		return nil, errRegistrationNotFound
	}

	if p.Status == PendingWemaStatusPending && (p.LastAttemptAt == nil || time.Since(*p.LastAttemptAt) >= pendingWemaPollRetryAfter) {
		providerCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		if _, err := s.tryCompletePendingWema(providerCtx, p.ID); err != nil {
			return nil, err
		}
		if p, err = s.repo.GetPendingWemaRegistration(ctx, id); err != nil || p == nil {
			return nil, fmt.Errorf("reload pending registration: %v", err)
		}
	}

	switch p.Status {
	case PendingWemaStatusPending, PendingWemaStatusProcessing:
		return &RegisterResponse{Status: RegistrationStatusPending, RegistrationID: p.ID}, nil
	case PendingWemaStatusFailed:
		return &RegisterResponse{Status: RegistrationStatusFailed, RegistrationID: p.ID,
			Message: "we could not complete your registration - please contact support"}, nil
	case PendingWemaStatusExpired:
		return &RegisterResponse{Status: RegistrationStatusExpired, RegistrationID: p.ID,
			Message: "your registration took too long to complete - please contact support"}, nil
	case PendingWemaStatusCompleted:
		claimed, err := s.repo.ClaimPendingWemaRegistration(ctx, p.ID, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		if !claimed {
			return nil, errRegistrationClaimed
		}
		return s.issueSession(ctx, rl, p.MobileUserID, p.DeviceID, ip)
	default:
		return nil, fmt.Errorf("pending registration has unknown status %q", p.Status)
	}
}

// OnAlatAccountCreated is called by ALAT's Account Creation webhook. The
// webhook only says "an account was created for this phone"; completion
// confirms it with ALAT before trusting it.
func (s *Service) OnAlatAccountCreated(ctx context.Context, phoneNumber string) {
	rl := newOpLogger("alat_account_created")
	var retErr error
	defer rl.finish(&retErr)

	phone, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(phoneNumber))
	if err != nil {
		rl.Warnf("webhook phone %s is not a valid nigerian number: %v", maskPhone(phoneNumber), err)
		return
	}
	rl.with("phone", maskPhone(phone))
	p, err := s.repo.GetActivePendingWemaRegistrationByPhone(ctx, phone)
	if err != nil {
		retErr = err
		return
	}
	if p == nil {
		rl.Infof("no pending registration for this phone - nothing to do")
		return
	}
	rl.with("registration_id", p.ID)
	if _, err := s.tryCompletePendingWema(ctx, p.ID); err != nil {
		retErr = err
	}
}

// SweepPendingWemaRegistrations is the backstop for missed or early
// webhooks: it frees rows stuck processing, expires overdue ones, and retries
// the rest.
func (s *Service) SweepPendingWemaRegistrations(ctx context.Context, limit int) error {
	now := time.Now().UTC()
	if n, err := s.repo.ResetStuckPendingWemaAttempts(ctx, now.Add(-pendingWemaStuckAfter)); err != nil {
		return err
	} else if n > 0 {
		newOpLogger("sweep_pending_wema").Warnf("returned %d stuck processing registration(s) to pending", n)
	}

	expired, err := s.repo.ExpirePendingWemaRegistrations(ctx, now)
	if err != nil {
		return err
	}
	for _, p := range expired {
		newOpLogger("sweep_pending_wema").with("registration_id", p.ID).with("phone", maskPhone(p.Phone)).
			Criticalf("pending registration expired without an alat account - check alat (tracking_id=%s); an account may still appear and need manual linking", p.TrackingID)
	}

	due, err := s.repo.ListDuePendingWemaRegistrations(ctx, now.Add(-pendingWemaSweepRetryAfter), limit)
	if err != nil {
		return err
	}
	for _, p := range due {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := s.tryCompletePendingWema(ctx, p.ID); err != nil {
			newOpLogger("sweep_pending_wema").with("registration_id", p.ID).Errorf("attempt failed: %v", err)
		}
	}
	return nil
}

func newClaimToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashClaimToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// logRejectedTier2Payload logs, redacted, the tier 2 payload this request
// would have sent to ALAT when the registration lock turned it away - so it
// can be compared with the attempt that holds the lock. Nothing is sent. The
// selfie is read from the face check named by the request (best effort) and
// logged by size only.
func (s *Service) logRejectedTier2Payload(ctx context.Context, rl *opLogger, req OptimusRegisterRequest, normalizedPhone, normalizedEmail string) {
	phone := normalizedPhone
	if local, err := phoneutil.ToLocalFormat(normalizedPhone); err == nil {
		phone = local
	}
	image := ""
	if id := strings.TrimSpace(req.BVNWithFaceVerificationID); id != "" && s.repo != nil && s.repo.db != nil {
		if fc, err := s.repo.GetFaceCheckRecord(ctx, id); err == nil && fc != nil && fc.FaceImageProvided != nil {
			image = strings.TrimSpace(*fc.FaceImageProvided)
		}
	}
	rl.Infof("rejected request would have sent to alat: tier2 payload=%s",
		logredact.Tier2Payload(strings.TrimSpace(req.BVN), strings.TrimSpace(req.NIN), phone, normalizedEmail, image, false, "not-sent"))
}
