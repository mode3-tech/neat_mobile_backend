package auth

import (
	"context"
	"errors"
	"neat_mobile_app_backend/internal/database/tx"
	"neat_mobile_app_backend/internal/notify"
	authotp "neat_mobile_app_backend/modules/auth/otp"
	"neat_mobile_app_backend/modules/auth/verification"
	"neat_mobile_app_backend/modules/device"

	"gorm.io/gorm"
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
	repo               *Repository
	coreCustomerFinder CoreCustomerFinder
	cbaCustomerUpdater CBACustomerUpdater
	verification       *verification.VerificationRepo
	tx                 *tx.Transactor
	deviceRepo         *device.Repository
	smsSender          notify.SMSSender
	otpPepper          string
	jwtSigner          JWTSigner
	tender             TendarValidation
	prembly            PremblyValidation
	nin                NINValidation
	providerSource     BVNProviderSource
	otpManager         authotp.OTPManager
	walletService      WalletService
	cbaSyncSem         chan struct{}
	cbaWalletUpdateSem chan struct{}
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
	cbaSyncSem, cbaWalletUpdateSem chan struct{},
) *Service {
	return &Service{
		repo:               repo,
		coreCustomerFinder: coreCustomerFinder,
		cbaCustomerUpdater: cbaCustomerUpdater,
		verification:       verification,
		tx:                 tx,
		deviceRepo:         deviceRepo,
		smsSender:          smsSender,
		otpPepper:          otpPepper,
		jwtSigner:          jwtSigner,
		tender:             tender,
		prembly:            prembly,
		nin:                nin,
		providerSource:     providerSource,
		otpManager:         otpManager,
		walletService:      walletService,
		cbaSyncSem:         cbaSyncSem,
		cbaWalletUpdateSem: cbaWalletUpdateSem,
	}
}

func (s *Service) ConfigureOTPManager(manager authotp.OTPManager) {
	s.otpManager = manager
}

func (s *Service) verifyUserDevice(ctx context.Context, userID, deviceID string) (*device.UserDevice, error) {
	if s.deviceRepo == nil {
		return nil, errors.New("device repository not configured")
	}
	rec, err := s.deviceRepo.FindDevice(ctx, userID, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("device not found")
		}
		return nil, err
	}
	if !rec.IsActive || !rec.IsTrusted {
		return nil, errors.New("device not allowed")
	}
	return rec, nil
}
