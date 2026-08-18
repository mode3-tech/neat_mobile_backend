package registerv2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/authchecker"
	"neat_mobile_app_backend/internal/database/tx"
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

type Service struct {
	repo               *Repository
	providerPreference ProviderPreferenceRepository
	optimus            OptimusValidator
	walletGenerator    auth.WalletService
	sessionIssuer      SessionIssuer
	otpManager         otp.OTPManager
	tx                 *tx.Transactor
	activationCapKobo  int64
	optimusProductID   string
}

func NewService(
	repo *Repository,
	providerPreference ProviderPreferenceRepository,
	optimus OptimusValidator,
	walletGenerator auth.WalletService,
	sessionIssuer SessionIssuer,
	otpManager otp.OTPManager,
	transactor *tx.Transactor,
	activationCapKobo int64,
	optimusProductID string,
) *Service {
	return &Service{
		repo:               repo,
		providerPreference: providerPreference,
		optimus:            optimus,
		walletGenerator:    walletGenerator,
		sessionIssuer:      sessionIssuer,
		otpManager:         otpManager,
		tx:                 transactor,
		activationCapKobo:  activationCapKobo,
		optimusProductID:   optimusProductID,
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

	bvnHash := sha256.Sum256([]byte(strings.TrimSpace(req.BVN)))
	bvnRecord, err := s.repo.GetVerifiedByTypeAndHash(ctx, models.VerificationTypeBVN, hex.EncodeToString(bvnHash[:]))
	if err != nil {
		return nil, err
	}
	if bvnRecord == nil {
		return nil, fmt.Errorf("bvn is not verified")
	}

	ninHash := sha256.Sum256([]byte(strings.TrimSpace(req.NIN)))
	ninRecord, err := s.repo.GetVerifiedByTypeAndHash(ctx, models.VerificationTypeNIN, hex.EncodeToString(ninHash[:]))
	if err != nil {
		return nil, err
	}
	if ninRecord == nil {
		return nil, fmt.Errorf("nin is not verified")
	}

	provider, err := s.CurrentProvider(ctx)
	if err != nil {
		return nil, err
	}

	switch strings.TrimSpace(provider) {
	case "optimus":
		if s.optimus == nil {
			log.Println("optimus service is not configured")
			return nil, fmt.Errorf("optimus service is not configured")
		}

	default:
		return nil, fmt.Errorf("registration provider %q is not supported by this flow", provider)
	}
	if s.walletGenerator == nil {
		return nil, fmt.Errorf("wallet generation service is not configured")
	}

	dob, err := time.Parse("2006-01-02", strings.TrimSpace(req.Dob))
	if err != nil {
		return nil, fmt.Errorf("invalid date of birth: %w", err)
	}

	mobileUserID := uuid.NewString()
	internalWalletID := uuid.NewString()

	walletResp, err := s.walletGenerator.GenerateWallet(ctx, &auth.WalletPayload{
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
	})
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
	usedVerificationIDs := []string{phoneRecord.ID, emailRecord.ID, bvnRecord.ID, ninRecord.ID}

	err = s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		authRepo := auth.NewRespository(txDB)
		walletRepo := wallet.NewRepository(txDB)
		deviceRepo := device.NewRepository(txDB)
		verificationRepo := &Repository{db: txDB}

		if _, txErr := authRepo.CreateUser(ctx, user); txErr != nil {
			return txErr
		}
		if txErr := authRepo.LinkBVNRecordToUser(ctx, user.BVN, user.ID); txErr != nil {
			return txErr
		}
		if txErr := walletRepo.CreateWallet(ctx, walletRecord); txErr != nil {
			return txErr
		}

		deviceService := device.NewService(*deviceRepo)
		if txErr := deviceService.BindDevice(ctx, mobileUserID, &device.DeviceBindingRequest{
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
			if txErr := verificationRepo.MarkVerificationUsed(ctx, id, now); txErr != nil {
				return txErr
			}
		}

		if strings.TrimSpace(req.ReferralCode) != "" {
			referralsRepo := referrals.NewRepository(txDB)
			if txErr := referrals.NewService(referralsRepo).RedeemReferralCode(ctx, mobileUserID, req.ReferralCode); txErr != nil {
				log.Printf("registerv2: referral redemption failed user=%s code=%s: %v", mobileUserID, req.ReferralCode, txErr)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.sessionIssuer == nil {
		return nil, fmt.Errorf("session issuer is not configured")
	}
	session, err := s.sessionIssuer.IssueSessionTokens(ctx, mobileUserID, req.Device.DeviceID, ip)
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

func (s *Service) ValidateBVN(ctx context.Context, request OptimusBVNValidationRequest) (string, string, error) {
	if s.optimus == nil {
		return "", "", fmt.Errorf("optimus validation service is not configured")
	}

	// RequestId is our correlation ID for this call, not client input -
	// always generated server-side, overwriting anything the caller sent.
	request.RequestId = uuid.NewString()

	response, err := s.optimus.ValidateBVN(ctx, request)
	if err != nil {
		return "", "", err
	}
	if response == nil || !isOptimusSuccess(response.ResponseCode) {
		return "", "", optimusResponseError(response)
	}

	// response.Data carries Optimus's own reference id for this check, which the
	// client needs later for /otp/verify and /otp/resend - store it as the
	// record's provider verification id rather than our own request.RequestId.
	providerReferenceID := strings.TrimSpace(response.Data)
	verificationID, err := s.createVerifiedIdentityVerification(ctx, models.VerificationTypeBVN, request.Bvn, providerReferenceID)
	if err != nil {
		return "", "", err
	}
	return verificationID, providerReferenceID, nil
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

	providerReferenceID := strings.TrimSpace(response.Data)
	verificationID, err := s.createVerifiedIdentityVerification(ctx, models.VerificationTypeNIN, request.Nin, providerReferenceID)
	if err != nil {
		return "", "", err
	}
	return verificationID, providerReferenceID, nil
}

// VerifyOTP confirms the OTP Optimus sent for referenceID. Generic - works for
// any Optimus OTP challenge (BVN/NIN validation today), since the reference id
// alone is what Optimus uses to identify which challenge this is.
func (s *Service) VerifyOTP(ctx context.Context, phone, otpToken, email, referenceID string) error {
	if s.optimus == nil {
		return fmt.Errorf("optimus validation service is not configured")
	}
	return s.optimus.VerifyOTPWithOptimus(ctx, phone, otpToken, email, referenceID)
}

// ResendOTP asks Optimus to resend the OTP tied to referenceID, returning the
// new reference id the caller must use for the next verify/resend.
func (s *Service) ResendOTP(ctx context.Context, referenceID string) (string, error) {
	if s.optimus == nil {
		return "", fmt.Errorf("optimus validation service is not configured")
	}
	return s.optimus.ResendOTPWithOptimus(ctx, referenceID)
}

func (s *Service) createVerifiedIdentityVerification(ctx context.Context, verificationType, identityNumber, providerReferenceID string) (string, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(verificationExpiry)
	hash := sha256.Sum256([]byte(identityNumber))
	masked := maskIdentityNumber(identityNumber)

	record := &models.VerificationRecord{
		ID:                     uuid.NewString(),
		Type:                   verificationType,
		Provider:               "optimus",
		Status:                 models.VerificationStatusVerified,
		SubjectHash:            hex.EncodeToString(hash[:]),
		SubjectMasked:          &masked,
		ProviderVerificationID: stringPointer(strings.TrimSpace(providerReferenceID)),
		VerifiedID:             &identityNumber,
		VerifiedAt:             &now,
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
