package auth

import (
	"context"
	"errors"
	"neat_mobile_app_backend/internal/validators"
	"neat_mobile_app_backend/models"
	authotp "neat_mobile_app_backend/modules/auth/otp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (s *Service) ForgotTransactionPin(ctx context.Context, mobileUserID, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("device id is required")
	}

	if s.otpManager == nil {
		return errors.New("otp manager not configured")
	}

	if _, err := s.deviceRepo.FindDevice(ctx, mobileUserID, deviceID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("no record of device found")
		}
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

	_, err = s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePinReset,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		UserID:      mobileUserID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})
	return err
}

func (s *Service) ResetTransactionPin(ctx context.Context, mobileUserID, deviceID string, req ResetTransactionPinRequest) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("device id is required")
	}

	otpCode := strings.TrimSpace(req.OTPCode)
	if otpCode == "" {
		return errors.New("otp code is required")
	}

	if err := validators.ValidatePin(req.NewPin); err != nil {
		return err
	}

	if req.NewPin != req.ConfirmNewPin {
		return errors.New("new pin and confirm new pin do not match")
	}

	if s.tx == nil {
		return errors.New("transaction manager not configured")
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		otpRepo := authotp.NewOTPRepository(txDB)

		if _, err := s.deviceRepo.FindDevice(ctx, mobileUserID, deviceID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("invalid device id")
			}
			return err
		}

		user, err := s.repo.GetUserByID(ctx, mobileUserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("user not found")
			}
			return err
		}

		normalizedPhone, err := NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return errors.New("invalid phone number on account")
		}

		activeOTP, err := otpRepo.GetActiveOTP(ctx, normalizedPhone, authotp.PurposePinReset)
		if err != nil {
			return err
		}
		if activeOTP == nil {
			return errors.New("invalid otp")
		}

		maxAttempts := activeOTP.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		if activeOTP.AttemptCount >= maxAttempts {
			return errors.New("invalid otp")
		}

		hashedOTP, err := authotp.HashOTP(s.otpPepper, authotp.PurposePinReset, normalizedPhone, otpCode)
		if err != nil {
			return errors.New("invalid otp")
		}

		if !authotp.HashEqualHex(hashedOTP, activeOTP.OTPHash) {
			if err := otpRepo.IncrementAttempt(ctx, activeOTP.ID); err != nil {
				return err
			}
			return errors.New("invalid otp")
		}

		now := time.Now().UTC()
		if err := otpRepo.ConsumeOTP(ctx, activeOTP.ID, now); err != nil {
			return err
		}

		hashedPin, err := HashPassword(req.NewPin)
		if err != nil {
			return err
		}

		return s.repo.UpdateUserPin(ctx, mobileUserID, hashedPin)
	})
}

func (s *Service) RequestTransactionPinChange(ctx context.Context, mobileUserID, deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return errors.New("mobile user id is required")
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

	_, err = s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePinChange,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		UserID:      mobileUserID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})
	return err
}

func (s *Service) ChangeTransactionPin(ctx context.Context, mobileUserID, deviceID string, req ChangeTransactionPinRequest) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return errors.New("mobile user id is required")
	}

	otpCode := strings.TrimSpace(req.OTPCode)
	if otpCode == "" {
		return errors.New("otp code is required")
	}

	if err := validators.ValidatePin(req.NewPin); err != nil {
		return err
	}

	if err := validators.ValidatePin(req.ConfirmNewPin); err != nil {
		return err
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

	if req.NewPin != req.ConfirmNewPin {
		return errors.New("new pin and confirm new pin do not match")
	}

	if s.otpManager == nil {
		return errors.New("otp manager not configured")
	}

	if s.tx == nil {
		return errors.New("transaction manager not configured")
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		otpRepo := authotp.NewOTPRepository(txDB)

		normalizedPhone, err := NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return errors.New("invalid phone number on account")
		}

		activeOTP, err := otpRepo.GetActiveOTP(ctx, normalizedPhone, authotp.PurposePinChange)
		if err != nil {
			return err
		}
		if activeOTP == nil {
			return errors.New("invalid otp")
		}

		maxAttempts := activeOTP.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		if activeOTP.AttemptCount >= maxAttempts {
			return errors.New("invalid otp")
		}

		hashedOTP, err := authotp.HashOTP(s.otpPepper, authotp.PurposePinChange, normalizedPhone, req.OTPCode)
		if err != nil {
			return errors.New("invalid otp")
		}

		if !authotp.HashEqualHex(hashedOTP, activeOTP.OTPHash) {
			if err := otpRepo.IncrementAttempt(ctx, activeOTP.ID); err != nil {
				return err
			}
			return errors.New("invalid otp")
		}

		now := time.Now().UTC()
		if err := otpRepo.ConsumeOTP(ctx, activeOTP.ID, now); err != nil {
			return err
		}

		hashedPin, err := HashPassword(req.NewPin)
		if err != nil {
			return err
		}

		return s.repo.UpdateUserPin(ctx, mobileUserID, hashedPin)
	})
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
