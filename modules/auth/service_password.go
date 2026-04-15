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

func (s *Service) RequestPasswordChange(ctx context.Context, mobileUserID, deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return errors.New("mobile user id is required")
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	phone, err := NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return errors.New("invalid phone number on account")
	}
	_, err = s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePasswordChange,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		UserID:      mobileUserID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})

	return err
}

func (s *Service) ResendPasswordChangeOTP(ctx context.Context, mobileUserID, deviceID string) error {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	phone, err := NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return errors.New("invalid phone number on account")
	}
	if _, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePasswordChange,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	}); err != nil {
		return err
	}

	return nil
}

func (s *Service) ChangePassword(ctx context.Context, mobileUserID, deviceID string, req ChangePasswordRequest) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return errors.New("mobile user id is required")
	}

	if strings.TrimSpace(req.OTPCode) == "" {
		return errors.New("otp code is required")
	}

	if err := validators.ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	if err := validators.ValidatePassword(req.ConfirmNewPassword); err != nil {
		return err
	}

	_, err := s.verifyUserDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	if err := s.validateCurrentPassword(user, req.CurrentPassword); err != nil {
		return err
	}

	if req.ConfirmNewPassword != req.NewPassword {
		return errors.New("new password and confirm new password do not match")
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

		activeOTP, err := otpRepo.GetActiveOTP(ctx, normalizedPhone, authotp.PurposePasswordChange)
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

		hashedOTP, err := authotp.HashOTP(s.otpPepper, authotp.PurposePasswordChange, normalizedPhone, req.OTPCode)
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

		hashedPassword, err := HashPassword(req.NewPassword)
		if err != nil {
			return err
		}

		return s.repo.UpdateUserPassword(ctx, mobileUserID, hashedPassword)
	})
}

func (s *Service) resolvePasswordResetTarget(ctx context.Context, phone string) (*models.User, string, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil, "", errors.New("phone is required")
	}

	normalizedPhone, err := NormalizeNigerianNumber(phone)
	if err != nil {
		return nil, "", errors.New(err.Error())
	}

	user, err := s.repo.GetUserByPhone(ctx, normalizedPhone)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", errors.New("no account exists under this phone number")
		}
		return nil, "", err
	}
	if user == nil {
		return nil, "", errors.New("no account exists under this phone number")
	}

	return user, normalizedPhone, nil
}

func (s *Service) issueForgotPasswordOTP(ctx context.Context, req ForgotPasswordRequest, deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}

	if s.otpManager == nil {
		return errors.New("otp manager not configured")
	}

	user, phone, err := s.resolvePasswordResetTarget(ctx, req.Phone)
	if err != nil {
		return err
	}

	if _, err := s.verifyUserDevice(ctx, user.ID, deviceID); err != nil {
		return err
	}

	_, err = s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePasswordReset,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		UserID:      user.ID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})
	return err
}

func (s *Service) ForgotPassword(ctx context.Context, req ForgotPasswordRequest, deviceID string) error {
	return s.issueForgotPasswordOTP(ctx, req, deviceID)
}

func (s *Service) ResendForgotPasswordOTP(ctx context.Context, req ForgotPasswordRequest, deviceID string) error {
	return s.issueForgotPasswordOTP(ctx, req, deviceID)
}

func (s *Service) ResetPassword(ctx context.Context, req ResetPasswordRequest, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("device id is required")
	}

	otpCode := strings.TrimSpace(req.OTPCode)
	if otpCode == "" {
		return errors.New("otp code is required")
	}

	if err := validators.ValidatePassword(req.NewPassword); err != nil {
		return errors.New(err.Error())
	}

	if err := validators.ValidatePassword(req.ConfirmNewPassword); err != nil {
		return errors.New(err.Error())
	}

	if req.NewPassword != req.ConfirmNewPassword {
		return errors.New("new password and confirm new password do not match")
	}

	user, normalizedPhone, err := s.resolvePasswordResetTarget(ctx, req.Phone)
	if err != nil {
		return err
	}

	if _, err := s.verifyUserDevice(ctx, user.ID, deviceID); err != nil {
		return err
	}

	hashedPassword, err := HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	if s.tx == nil {
		return errors.New("transaction manager not configured")
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		otpRepo := authotp.NewOTPRepository(txDB)

		activeOTP, err := otpRepo.GetActiveOTP(ctx, normalizedPhone, authotp.PurposePasswordReset)
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

		hashedCode, err := authotp.HashOTP(s.otpPepper, authotp.PurposePasswordReset, normalizedPhone, otpCode)
		if err != nil {
			return errors.New("invalid otp")
		}
		if !authotp.HashEqualHex(hashedCode, activeOTP.OTPHash) {
			if err := otpRepo.IncrementAttempt(ctx, activeOTP.ID); err != nil {
				return err
			}
			return errors.New("invalid otp")
		}

		now := time.Now().UTC()
		if err := otpRepo.ConsumeOTP(ctx, activeOTP.ID, now); err != nil {
			return err
		}

		result := txDB.WithContext(ctx).
			Model(&models.User{}).
			Where("id = ?", user.ID).
			Update("password_hash", hashedPassword)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("no account exists under this phone number")
		}

		return nil
	})
}

func (s *Service) validateCurrentPassword(user *models.User, currentPassword string) error {
	if user == nil {
		return errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(currentPassword),
	); err != nil {
		return errors.New("invalid current password")
	}
	return nil
}
