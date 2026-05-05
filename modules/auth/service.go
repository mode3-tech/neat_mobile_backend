package auth

import (
	"neat_mobile_app_backend/internal/database/tx"
	"neat_mobile_app_backend/internal/notify"
	authotp "neat_mobile_app_backend/modules/auth/otp"
	"neat_mobile_app_backend/modules/auth/verification"
	"neat_mobile_app_backend/modules/device"
)

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
	}
}

func (s *Service) ConfigureOTPManager(manager authotp.OTPManager) {
	s.otpManager = manager
}
