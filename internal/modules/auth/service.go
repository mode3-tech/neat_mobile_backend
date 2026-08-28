package auth

import (
	"context"
	"fmt"
	"neat_mobile_app_backend/internal/database/tx"
	appErr "neat_mobile_app_backend/internal/errors"
	authotp "neat_mobile_app_backend/internal/modules/auth/otp"
	"neat_mobile_app_backend/internal/modules/auth/verification"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/modules/referrals"
	"neat_mobile_app_backend/internal/notify"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type bvnInfo struct {
	name           string
	dob            string
	phone          string
	email          string
	verificationID string
}

type bvnWithFaceInfo struct {
	faceCheckID string
}

type bvnFaceValidationResult struct {
	Matched           bool
	Confidence        float64
	Message           string
	ResponseCode      string
	FaceImageProvided string
	ReferenceID       string
	TransactionID     string
}

type ninWithFaceInfo struct {
	faceCheckID string
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
	maxPinAttempts  = 5
	pinLockDuration = 30 * time.Minute
)

type Service struct {
	repo                 *Repository
	coreCustomerFinder   CoreCustomerFinder
	cbaCustomerUpdater   CBACustomerUpdater
	verification         *verification.VerificationRepo
	tx                   *tx.Transactor
	deviceRepo           *device.Repository
	smsSender            notify.SMSSender
	otpPepper            string
	jwtSigner            JWTSigner
	tender               TendarValidation
	prembly              PremblyValidation
	ninPrembly           NINValidation
	ninTendar            NINValidation
	ninFace              NINFaceValidation
	ninFaceTendar        NINFaceValidationTendar
	bvnFaceTendar        BVNFaceValidationTendar
	providerSource       ValidationProviderSource
	otpManager           authotp.OTPManager
	walletService        WalletService
	walletPayloadSeedKey string
	deviceVerifier       DeviceVerifier
	cbaSyncSem           chan struct{}
	cbaWalletUpdateSem   chan struct{}
	productID            string
	optimusKYC           OptimusKYCValidation
	activationCapKobo    int64
	referralsRepo        *referrals.Repository
	// walletProviderName is which BaaS provider ("optimus" or "providus")
	// walletService is actually wired to for this legacy async registration
	// flow - unlike registerv2, there's no per-request fallback here, so
	// whichever provider was configured at startup is the one every wallet
	// this flow creates gets stamped with.
	walletProviderName string
}

func NewService(
	repo *Repository,
	coreCustomerFinder CoreCustomerFinder,
	cbaCustomerUpdater CBACustomerUpdater,
	verification *verification.VerificationRepo,
	tx *tx.Transactor,
	deviceRepo *device.Repository,
	smsSender notify.SMSSender,
	otpPepper string,
	jwtSigner JWTSigner,
	tender TendarValidation,
	prembly PremblyValidation,
	ninPrembly NINValidation,
	ninTendar NINValidation,
	ninFace NINFaceValidation,
	ninFaceTendar NINFaceValidationTendar,
	bvnFaceTendar BVNFaceValidationTendar,
	providerSource ValidationProviderSource,
	otpManager authotp.OTPManager,
	walletService WalletService,
	walletPayloadSeedKey string,
	deviceVerifier DeviceVerifier,
	cbaSyncSem, cbaWalletUpdateSem chan struct{},
	productID string,
	activationCapKobo int64,
	walletProviderName string,
) *Service {
	return &Service{
		repo:                 repo,
		coreCustomerFinder:   coreCustomerFinder,
		cbaCustomerUpdater:   cbaCustomerUpdater,
		verification:         verification,
		tx:                   tx,
		deviceRepo:           deviceRepo,
		smsSender:            smsSender,
		otpPepper:            otpPepper,
		jwtSigner:            jwtSigner,
		tender:               tender,
		prembly:              prembly,
		ninPrembly:           ninPrembly,
		ninTendar:            ninTendar,
		ninFace:              ninFace,
		ninFaceTendar:        ninFaceTendar,
		bvnFaceTendar:        bvnFaceTendar,
		providerSource:       providerSource,
		otpManager:           otpManager,
		walletService:        walletService,
		walletPayloadSeedKey: walletPayloadSeedKey,
		deviceVerifier:       deviceVerifier,
		cbaSyncSem:           cbaSyncSem,
		cbaWalletUpdateSem:   cbaWalletUpdateSem,
		productID:            productID,
		activationCapKobo:    activationCapKobo,
		walletProviderName:   walletProviderName,
	}
}

func (s *Service) ConfigureOTPManager(manager authotp.OTPManager) {
	s.otpManager = manager
}

func (s *Service) ConfigureOptimusKYC(kyc OptimusKYCValidation) {
	s.optimusKYC = kyc
}

func (s *Service) ConfigureReferralsRepo(repo *referrals.Repository) {
	s.referralsRepo = repo
}

func (s *Service) VerifyTransactionPin(ctx context.Context, mobileUserID, pin string) error {
	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		return appErr.ErrIncorrectTransactionPin
	}

	if user.TransactionPinLockedUntil != nil && user.TransactionPinLockedUntil.After(time.Now().UTC()) {
		return appErr.ErrTransactionPinLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(pin)); err != nil {
		newAttempts := user.FailedTransactionPinAttempts + 1
		if newAttempts >= maxPinAttempts {
			_ = s.repo.LockTransactionPin(ctx, mobileUserID, time.Now().UTC().Add(pinLockDuration))
			return appErr.ErrTransactionPinLocked
		}

		_ = s.repo.IncrementFailedPinAttempts(ctx, mobileUserID)
		return fmt.Errorf("%w: you have %d attempt(s) left", appErr.ErrIncorrectTransactionPin, maxPinAttempts-newAttempts)
	}

	_ = s.repo.ResetPinAttempts(ctx, mobileUserID)
	return nil
}
