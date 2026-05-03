package auth

import (
	"context"
	"errors"
	appErr "neat_mobile_app_backend/internal/errors"
	phoneutil "neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/internal/validators"
	"neat_mobile_app_backend/models"
	authotp "neat_mobile_app_backend/modules/auth/otp"
	"neat_mobile_app_backend/modules/auth/verification"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (s *Service) ForgotTransactionPin(ctx context.Context, mobileUserID, deviceID string) (*ForgotTransactionPinResponse, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}

	if s.otpManager == nil {
		return nil, errors.New("otp manager not configured")
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrUnauthorized
	}

	phone, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return nil, appErr.ErrInvalidPhone
	}

	result, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePinReset,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		UserID:      mobileUserID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})

	if err != nil {
		return nil, err
	}

	return &ForgotTransactionPinResponse{
		Status:  "success",
		Message: "OTP has been sent to your phone",
		OTPID:   result.OTPID,
	}, err
}

func (s *Service) VerifyForgotTransactionPinOTP(ctx context.Context, mobileUserID, deviceID string, req VerifyForgotTransactionPinOTPRequest) (*VerifyForgotTransactionPinOTPResponse, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return nil, errors.New("mobile user is required")
	}

	if strings.TrimSpace(req.OTPID) == "" {
		return nil, errors.New("otp id is required")
	}

	if strings.TrimSpace(req.OTPCode) == "" {
		return nil, errors.New("otp code is required")
	}

	if s.otpManager == nil {
		return nil, errors.New("otp manager not configured")
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	result, err := s.otpManager.Verify(ctx, authotp.VerifyOTPInput{
		Purpose: authotp.PurposePinReset,
		OTPID:   strings.TrimSpace(req.OTPID),
		Code:    strings.TrimSpace(req.OTPCode),
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("otp verification failed")
	}

	return &VerifyForgotTransactionPinOTPResponse{
		Status:         "success",
		Message:        "OTP verified successfully",
		VerificationID: result.VerificationID,
	}, nil
}

func (s *Service) ResetTransactionPin(ctx context.Context, mobileUserID, deviceID string, req ResetTransactionPinRequest) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return errors.New("mobile user id is required")
	}

	if strings.TrimSpace(req.VerificationID) == "" {
		return errors.New("verification id is required")
	}

	if err := validators.ValidatePin(req.NewPin); err != nil {
		return err
	}

	if req.NewPin != req.ConfirmNewPin {
		return appErr.ErrTransactionPinMismatch
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appErr.ErrUnauthorized
		}
		return err
	}

	if s.tx == nil {
		return errors.New("transaction manager not configured")
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		verRepo := verification.NewVerification(txDB)
		serviceRepo := NewRespository(txDB)

		normalizedPhone, err := phoneutil.NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return appErr.ErrInvalidPhone
		}
		rec, err := verRepo.GetVerificationByID(ctx, strings.TrimSpace(req.VerificationID))
		if err != nil {
			return err
		}

		if rec == nil {
			return appErr.ErrUnauthorized
		}

		if rec.Status != models.VerificationStatusVerified {
			return appErr.ErrUnauthorized
		}

		if rec.VerifiedPhone == nil || rec.VerifiedPhone != &normalizedPhone {
			return appErr.ErrUnauthorized
		}

		now := time.Now().UTC()
		if rec.ExpiresAt == nil || now.After(*rec.ExpiresAt) {
			return appErr.ErrUnauthorized
		}

		if err := verRepo.MarkVerificationUsed(ctx, rec.ID, now); err != nil {
			return appErr.ErrUnauthorized
		}

		hashedPin, err := HashPassword(req.NewPin)
		if err != nil {
			return err
		}

		return serviceRepo.UpdateUserPin(ctx, mobileUserID, hashedPin)
	})
}

func (s *Service) ResendForgotTransactionPinOTP(ctx context.Context, mobileUserID, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("device id is required")
	}

	if s.otpManager == nil {
		return errors.New("otp manager not configured")
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		return appErr.ErrUnauthorized
	}

	phone, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return appErr.ErrInvalidPhone
	}

	if _, err = s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePinReset,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		UserID:      mobileUserID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	}); err != nil {
		return err
	}

	return nil
}

func (s *Service) RequestTransactionPinChange(ctx context.Context, mobileUserID, deviceID string) (*RequestTransactionPinChangeResponse, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return nil, errors.New("mobile user id is required")
	}

	if s.otpManager == nil {
		return nil, errors.New("otp manager not configured")
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrUnauthorized
	}

	phone, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return nil, appErr.ErrInvalidPhone
	}

	result, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePinChange,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		UserID:      mobileUserID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})
	if err != nil {
		return nil, err
	}

	return &RequestTransactionPinChangeResponse{
		Message: "OTP has been sent to your phone",
		OTPID:   result.OTPID,
	}, nil
}

func (s *Service) VerifyTransactionPinChangeOTP(ctx context.Context, mobileUserID, deviceID string, req VerifyTransactionPinChangeOTPRequest) (*VerifyTransactionPinChangeOTPResponse, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return nil, errors.New("mobile user id is required")
	}

	if strings.TrimSpace(req.OTPID) == "" {
		return nil, errors.New("otp id is required")
	}

	if strings.TrimSpace(req.OTPCode) == "" {
		return nil, errors.New("otp code is required")
	}

	if s.otpManager == nil {
		return nil, errors.New("otp manager not configured")
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	result, err := s.otpManager.Verify(ctx, authotp.VerifyOTPInput{
		Purpose: authotp.PurposePinChange,
		OTPID:   strings.TrimSpace(req.OTPID),
		Code:    strings.TrimSpace(req.OTPCode),
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, appErr.ErrInvalidOTP
	}

	return &VerifyTransactionPinChangeOTPResponse{
		Message:        "OTP verified successfully",
		VerificationID: result.VerificationID,
	}, nil
}

func (s *Service) ChangeTransactionPin(ctx context.Context, mobileUserID, deviceID string, req ChangeTransactionPinRequest) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return errors.New("mobile user id is required")
	}

	if strings.TrimSpace(req.VerificationID) == "" {
		return errors.New("verification id is required")
	}

	if err := validators.ValidatePin(req.NewPin); err != nil {
		return err
	}

	if err := validators.ValidatePin(req.ConfirmNewPin); err != nil {
		return err
	}

	if req.NewPin != req.ConfirmNewPin {
		return appErr.ErrTransactionPinMismatch
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appErr.ErrUnauthorized
		}
		return err
	}

	if err := s.validateCurrentTransactionPin(user, req.CurrentPin); err != nil {
		return err
	}

	if s.tx == nil {
		return errors.New("transaction manager not configured")
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		verRepo := verification.NewVerification(txDB)
		serviceRepo := NewRespository(txDB)
		rec, err := verRepo.GetVerificationByID(ctx, strings.TrimSpace(req.VerificationID))
		if err != nil {
			return err
		}
		if rec == nil {
			return appErr.ErrUnauthorized
		}

		if rec.Status != models.VerificationStatusVerified {
			return appErr.ErrUnauthorized
		}

		normalizedPhone, err := phoneutil.NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return appErr.ErrInvalidPhone
		}
		if rec.VerifiedPhone == nil || *rec.VerifiedPhone != normalizedPhone {
			return appErr.ErrUnauthorized
		}

		now := time.Now().UTC()
		if rec.ExpiresAt == nil || now.After(*rec.ExpiresAt) {
			return appErr.ErrUnauthorized
		}

		if err := verRepo.MarkVerificationUsed(ctx, rec.ID, now); err != nil {
			return appErr.ErrUnauthorized
		}

		hashedPin, err := HashPassword(req.NewPin)
		if err != nil {
			return err
		}

		return serviceRepo.UpdateUserPin(ctx, mobileUserID, hashedPin)
	})
}

func (s *Service) ResendTransactionPinChangeOTP(ctx context.Context, mobileUserID, deviceID string) (*ResendTransactionPinChangeOTPResponse, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return nil, errors.New("mobile user id is required")
	}

	if s.otpManager == nil {
		return nil, errors.New("otp manager not configured")
	}

	if _, err := s.deviceVerifier.VerifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrUnauthorized
	}

	phone, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return nil, appErr.ErrInvalidPhone
	}

	result, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePinChange,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		UserID:      mobileUserID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})
	if err != nil {
		return nil, err
	}

	return &ResendTransactionPinChangeOTPResponse{
		Message: "OTP resent successfully",
		OTPID:   result.OTPID,
	}, nil
}

func (s *Service) validateCurrentTransactionPin(user *models.User, currentTransactionPin string) error {
	if user == nil {
		return appErr.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PinHash),
		[]byte(currentTransactionPin),
	); err != nil {
		return appErr.ErrInvalidCredentials
	}
	return nil
}
