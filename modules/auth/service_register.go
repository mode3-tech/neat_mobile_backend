package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"neat_mobile_app_backend/internal/timeutil"
	"neat_mobile_app_backend/internal/validators"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/device"
	"neat_mobile_app_backend/modules/wallet"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) Register(ctx context.Context, req RegisterRequest, ip string) (*AuthObject, error) {
	now := time.Now().UTC()
	mobileUserID := uuid.NewString()
	internalWalletID := uuid.NewString()

	// Pre-fetch verification records to build the Providus payload before any DB writes
	bvnRecord, err := s.repo.GetValidationRow(ctx, req.BVNVerificationID)
	if err != nil || bvnRecord.VerifiedName == nil || bvnRecord.VerifiedID == nil {
		return nil, errors.New("bvn verification record not found")
	}

	err = s.repo.MarkValidationRecordUsed(ctx, req.BVNVerificationID)
	if err != nil {
		return nil, errors.New("failed to mark bvn verification record as used")
	}

	ninRecord, err := s.repo.GetValidationRow(ctx, req.NINVerificationID)
	if err != nil || ninRecord.VerifiedDOB == nil {
		return nil, errors.New("nin verification record not found")
	}

	err = s.repo.MarkValidationRecordUsed(ctx, req.NINVerificationID)
	if err != nil {
		return nil, errors.New("failed to mark nin verification record as used")
	}

	normalizedPhone, err := NormalizeNigerianNumber(req.PhoneNumber)
	if err != nil {
		return nil, err
	}

	dobParsed, err := timeutil.ParseDOB(*ninRecord.VerifiedDOB)
	if err != nil {
		return nil, errors.New(err.Error())
	}

	firstName, _, lastName := SplitFullName(*bvnRecord.VerifiedName)

	walletInfo := &WalletPayload{
		BVN:         *bvnRecord.VerifiedID,
		FirstName:   firstName,
		LastName:    lastName,
		DateOfBirth: dobParsed.Format("2006-01-02"),
		PhoneNumber: normalizedPhone,
		Email:       req.Email,
		Metadata:    map[string]interface{}{"customer_id": mobileUserID},
	}

	walletResp, err := s.walletService.GenerateWallet(ctx, walletInfo)
	if err != nil {
		return nil, err
	}

	err = s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		authRepo := NewRespository(txDB)
		deviceRepo := device.NewRepository(txDB)
		walletRepo := wallet.NewRepository(txDB)

		_, txErr := s.createUser(ctx, authRepo, req, mobileUserID, internalWalletID)
		if txErr != nil {
			return txErr
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
			Address:          *walletResp.Customer.Address,
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
			UpdatedAt:        nil,
		}

		if txErr = walletRepo.CreateWallet(ctx, walletRecord); txErr != nil {
			return txErr
		}

		deviceReq := device.DeviceBindingRequest{
			DeviceID:    req.Device.DeviceID,
			PublicKey:   req.Device.PublicKey,
			DeviceName:  req.Device.DeviceName,
			DeviceModel: req.Device.DeviceModel,
			OS:          req.Device.OS,
			OSVersion:   req.Device.OSVersion,
			AppVersion:  req.Device.AppVersion,
			IP:          ip,
		}
		deviceService := device.NewDeviceService(*deviceRepo)
		return deviceService.BindDevice(ctx, mobileUserID, &deviceReq)
	})

	if err != nil {
		return nil, err
	}

	sid := uuid.NewString()

	accessToken, err := s.jwtSigner.IssueAccessToken(mobileUserID, sid)
	if err != nil {
		return nil, err
	}

	authSession := &models.AuthSession{
		UserID:   mobileUserID,
		SID:      sid,
		DeviceID: &req.Device.DeviceID,
		IP:       &ip,
	}

	if err = s.repo.AddAccessToken(ctx, authSession); err != nil {
		return nil, err
	}

	refreshToken, jti, _, err := s.jwtSigner.IssueRefreshToken(mobileUserID, sid)
	if err != nil {
		return nil, err
	}

	hashedRefreshToken := sha256.Sum256([]byte(refreshToken))

	refreshTokenObj := &models.RefreshToken{
		JTI:       jti,
		SessionID: sid,
		UserID:    mobileUserID,
		TokenHash: hex.EncodeToString(hashedRefreshToken[:]),
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour * 24 * 30),
	}

	if err := s.repo.AddRefreshToken(ctx, refreshTokenObj); err != nil {
		return nil, err
	}

	go s.syncAndUpdateCBACustomer(context.Background(), mobileUserID, *bvnRecord.VerifiedID, walletResp.Wallet.AccountName, walletResp.Wallet.AccountNumber, walletResp.Wallet.BankCode, walletResp.Wallet.BankName)

	return &AuthObject{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *Service) createUser(ctx context.Context, repo *Repository, req RegisterRequest, mobileUserID, internalWalletID string) (*models.User, error) {
	var isEmailVerified bool
	phoneRecord, err := repo.GetValidationRow(ctx, req.PhoneVerificationID)
	if err != nil {
		return nil, errors.New("phone verification record not found")
	}

	if err := s.repo.MarkValidationRecordUsed(ctx, phoneRecord.ID); err != nil {
		return nil, errors.New("failed to mark phone verification record as used")
	}

	normalizedNumber, err := NormalizeNigerianNumber(req.PhoneNumber)
	if err != nil {
		return nil, errors.New(err.Error())
	}

	if *phoneRecord.VerifiedPhone != normalizedNumber {
		return nil, errors.New("phone number does not match")
	}

	existingUser, err := repo.GetUserByPhone(ctx, normalizedNumber)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existingUser != nil {
		return nil, errors.New("user already exists")
	}

	bvnRecord, err := repo.GetValidationRow(ctx, req.BVNVerificationID)
	if err != nil || bvnRecord.VerifiedName == nil || bvnRecord.VerifiedDOB == nil {
		return nil, errors.New("bvn verification record not found")
	}

	if err := s.repo.MarkValidationRecordUsed(ctx, bvnRecord.ID); err != nil {
		return nil, errors.New("failed to mark bvn verification record as used")
	}

	ninRecord, err := repo.GetValidationRow(ctx, req.NINVerificationID)
	if err != nil || ninRecord.VerifiedName == nil || ninRecord.VerifiedDOB == nil {
		return nil, errors.New("nin verification record not found")
	}

	if err := s.repo.MarkValidationRecordUsed(ctx, ninRecord.ID); err != nil {
		return nil, errors.New("failed to mark nin verification record as used")
	}

	if req.Email != "" {
		emailRecord, err := repo.GetValidationRow(ctx, req.EmailVerificationID)
		if err != nil || emailRecord.VerifiedName == nil || emailRecord.VerifiedDOB == nil {
			return nil, errors.New("email verification record not found")
		}

		if emailRecord.VerifiedName != phoneRecord.VerifiedName || emailRecord.VerifiedDOB != phoneRecord.VerifiedDOB {
			return nil, errors.New("unable to confirm email and phone number belong to the same person due to names or date of births mismatch")
		}

		isEmailVerified = true

		if err := s.repo.MarkValidationRecordUsed(ctx, emailRecord.ID); err != nil {
			return nil, errors.New("failed to mark email verification record as used")
		}
	}

	bvnName := strings.ToLower(strings.Join(strings.Fields(*bvnRecord.VerifiedName), " "))
	ninName := strings.ToLower(strings.Join(strings.Fields(*ninRecord.VerifiedName), " "))

	if bvnName != ninName || SerializeDOB(*bvnRecord.VerifiedDOB) != SerializeDOB(*ninRecord.VerifiedDOB) {
		return nil, errors.New("unable to confirm bvn and nin belong to the same person due to names or date of births mismatch")
	}

	if req.Password != req.ConfirmPassword {
		return nil, errors.New("passwords do not match")
	}

	if err = validators.ValidatePassword(req.Password); err != nil {
		return nil, errors.New(err.Error())
	}

	if req.TransactionPin != req.ConfirmTransactionPin {
		return nil, errors.New("transaction pins do not match")
	}

	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	hashedTransactionPin, err := HashPassword(req.TransactionPin)
	if err != nil {
		return nil, err
	}

	normalizedPhone, err := NormalizeNigerianNumber(req.PhoneNumber)
	if err != nil {
		return nil, err
	}

	dob, err := timeutil.ParseDOB(*ninRecord.VerifiedDOB)

	if err != nil {
		return nil, errors.New(err.Error())
	}

	firstName, middleName, lastName := SplitFullName(*bvnRecord.VerifiedName)

	isPhoneVerified := phoneRecord.VerifiedPhone == &normalizedNumber
	isBvnVerified := bvnRecord != nil
	isNinVerified := ninRecord != nil

	user := &models.User{
		ID:                  mobileUserID,
		WalletID:            internalWalletID,
		Phone:               normalizedPhone,
		Email:               &req.Email,
		FirstName:           firstName,
		LastName:            lastName,
		MiddleName:          &middleName,
		PasswordHash:        hashedPassword,
		PinHash:             hashedTransactionPin,
		IsEmailVerified:     isEmailVerified,
		BVN:                 *bvnRecord.VerifiedID,
		NIN:                 *ninRecord.VerifiedID,
		DOB:                 dob,
		IsPhoneVerified:     isPhoneVerified,
		IsBvnVerified:       isBvnVerified,
		IsNinVerified:       isNinVerified,
		IsBiometricsEnabled: req.IsBiometricsEnabled,
	}

	createdUser, err := repo.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	if err := repo.LinkBVNRecordToUser(ctx, user.BVN, createdUser.ID); err != nil {
		return nil, err
	}

	return createdUser, nil
}
