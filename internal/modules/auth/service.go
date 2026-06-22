package auth

import (
	"context"
	"fmt"
	"neat_mobile_app_backend/internal/database/tx"
	appErr "neat_mobile_app_backend/internal/errors"
	authotp "neat_mobile_app_backend/internal/modules/auth/otp"
	"neat_mobile_app_backend/internal/modules/auth/verification"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/notify"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type bvnInfo struct {
	name           string
	dob            string
	phone          string
	verificationID string
}

type bvnWithFaceInfo struct {
	faceCheckID string
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
	nin                  NINValidation
	providerSource       BVNProviderSource
	otpManager           authotp.OTPManager
	walletService        WalletService
	walletPayloadSeedKey string
	deviceVerifier       DeviceVerifier
	cbaSyncSem           chan struct{}
	cbaWalletUpdateSem   chan struct{}
	productID            string
	optimusKYC           OptimusKYCValidation
	activationCapKobo    int64
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
	nin NINValidation,
	providerSource BVNProviderSource,
	otpManager authotp.OTPManager,
	walletService WalletService,
	walletPayloadSeedKey string,
	deviceVerifier DeviceVerifier,
	cbaSyncSem, cbaWalletUpdateSem chan struct{},
	productID string,
	activationCapKobo int64,
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
		nin:                  nin,
		providerSource:       providerSource,
		otpManager:           otpManager,
		walletService:        walletService,
		walletPayloadSeedKey: walletPayloadSeedKey,
		deviceVerifier:       deviceVerifier,
		cbaSyncSem:           cbaSyncSem,
		cbaWalletUpdateSem:   cbaWalletUpdateSem,
		productID:            productID,
		activationCapKobo:    activationCapKobo,
	}
}

func (s *Service) ConfigureOTPManager(manager authotp.OTPManager) {
	s.otpManager = manager
}

func (s *Service) ConfigureOptimusKYC(kyc OptimusKYCValidation) {
	s.optimusKYC = kyc
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
