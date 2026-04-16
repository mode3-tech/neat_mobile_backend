package auth

import (
	"context"
	"errors"
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

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	phone, err := NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return nil, errors.New("invalid phone number on account")
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

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
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
		return nil, errors.New("otp verification failed")
	}

	return &VerifyForgotTransactionPinOTPResponse{
		Status:         "succes",
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
		return errors.New("new pin and confirm new pin do not match")
	}

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	if s.tx == nil {
		return errors.New("transaction manager not configured")
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		verRepo := verification.NewVerification(txDB)
		serviceRepo := NewRespository(txDB)

		normalizedPhone, err := NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return errors.New("invalid phone number on account")
		}
		rec, err := verRepo.GetVerificationByID(ctx, strings.TrimSpace(req.VerificationID))
		if err != nil {
			return err
		}

		if rec == nil {
			return errors.New("invalid verification id")
		}

		if rec.Status != models.VerificationStatusVerified {
			return errors.New("invalid verification id")
		}

		if rec.VerifiedPhone == nil || rec.VerifiedPhone != &normalizedPhone {
			return errors.New("invalid verification id")
		}

		now := time.Now().UTC()
		if rec.ExpiresAt == nil || now.After(*rec.ExpiresAt) {
			return errors.New("verification id has expired")
		}

		if err := verRepo.MarkVerificationUsed(ctx, rec.ID, now); err != nil {
			return errors.New("invalid verification id")
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

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		return errors.New("user not found")
	}

	phone, err := NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return errors.New("invalid phone number on account")
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

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	phone, err := NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return nil, errors.New("invalid phone number on account")
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

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
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
		return nil, errors.New("otp verification failed")
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
		return errors.New("new pin and confirm new pin do not match")
	}

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
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
			return errors.New("invalid verification id")
		}

		if rec.Status != models.VerificationStatusVerified {
			return errors.New("invalid verification id")
		}

		normalizedPhone, err := NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return errors.New("invalid phone number on account")
		}
		if rec.VerifiedPhone == nil || *rec.VerifiedPhone != normalizedPhone {
			return errors.New("invalid verification id")
		}

		now := time.Now().UTC()
		if rec.ExpiresAt == nil || now.After(*rec.ExpiresAt) {
			return errors.New("verification id has expired")
		}

		if err := verRepo.MarkVerificationUsed(ctx, rec.ID, now); err != nil {
			return errors.New("invalid verification id")
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

	if _, err := s.verifyUserDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	phone, err := NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return nil, errors.New("invalid phone number on account")
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
		return errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PinHash),
		[]byte(currentTransactionPin),
	); err != nil {
		return errors.New("invalid current pin")
	}
	return nil
}
