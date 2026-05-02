package auth

import (
	"context"
	"errors"
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

func (s *Service) RequestPasswordChange(ctx context.Context, mobileUserID, deviceID string) (*RequestChangePasswordResponse, error) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	phone, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return nil, errors.New("invalid phone number on account")
	}
	result, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePasswordChange,
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

	return &RequestChangePasswordResponse{
		Status:  "success",
		Message: "OTP has been sent to your phone",
		OTPID:   result.OTPID,
	}, nil
}

func (s *Service) VerifyPasswordChangeOTP(ctx context.Context, mobileUserID, deviceID string, req VerifyPasswordChangeOTPRequest) (*VerifyPasswordChangeOTPResponse, error) {
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
		Purpose: authotp.PurposePasswordChange,
		OTPID:   strings.TrimSpace(req.OTPID),
		Code:    strings.TrimSpace(req.OTPCode),
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("otp verification failed")
	}

	return &VerifyPasswordChangeOTPResponse{
		Status:         "success",
		Message:        "OTP verified successfully",
		VerificationID: result.VerificationID,
	}, nil
}

func (s *Service) ChangePassword(ctx context.Context, mobileUserID, deviceID string, req ChangePasswordRequest) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}

	if strings.TrimSpace(mobileUserID) == "" {
		return errors.New("mobile user id is required")
	}

	if strings.TrimSpace(req.VerificationID) == "" {
		return errors.New("verification id is required")
	}

	if err := validators.ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	if err := validators.ValidatePassword(req.ConfirmNewPassword); err != nil {
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

	if err := s.validateCurrentPassword(user, req.CurrentPassword); err != nil {
		return err
	}

	if req.ConfirmNewPassword != req.NewPassword {
		return errors.New("new password and confirm new password do not match")
	}

	if s.tx == nil {
		return errors.New("transaction manager not configured")
	}

	return s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		verRepo := verification.NewVerification(txDB)
		serviceRepo := NewRespository(txDB)

		normalizedPhone, err := phoneutil.NormalizeNigerianNumber(user.Phone)
		if err != nil {
			return errors.New("invalid phone number on account")
		}

		rec, err := verRepo.GetVerificationByID(ctx, req.VerificationID)
		if err != nil {
			return err
		}

		if rec == nil {
			return errors.New("invalid verification id")
		}

		if rec.Status != models.VerificationStatusVerified {
			return errors.New("invalid verification id")
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

		hashedPassword, err := HashPassword(req.NewPassword)
		if err != nil {
			return err
		}

		return serviceRepo.UpdateUserPassword(ctx, mobileUserID, hashedPassword)
	})
}

func (s *Service) ResendPasswordChangeOTP(ctx context.Context, mobileUserID, deviceID string) (*ResendPasswordChangeOTPResponse, error) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	phone, err := phoneutil.NormalizeNigerianNumber(strings.TrimSpace(user.Phone))
	if err != nil {
		return nil, errors.New("invalid phone number on account")
	}

	result, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePasswordChange,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})
	if err != nil {
		return nil, err
	}

	return &ResendPasswordChangeOTPResponse{
		Message: "OTP resent successfully",
		OTPID:   result.OTPID,
	}, nil
}

func (s *Service) resolvePasswordResetTarget(ctx context.Context, phone string) (*models.User, string, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil, "", errors.New("phone is required")
	}

	normalizedPhone, err := phoneutil.NormalizeNigerianNumber(phone)
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

func (s *Service) issueForgotPasswordOTP(ctx context.Context, req ForgotPasswordRequest, deviceID string) (*ForgotPasswordResponse, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, errors.New("device id is required")
	}

	if s.otpManager == nil {
		return nil, errors.New("otp manager not configured")
	}

	user, phone, err := s.resolvePasswordResetTarget(ctx, req.Phone)
	if err != nil {
		return nil, err
	}

	result, err := s.otpManager.Issue(ctx, authotp.IssueOTPInput{
		Purpose:     authotp.PurposePasswordReset,
		Channel:     authotp.ChannelSMS,
		Destination: phone,
		UserID:      user.ID,
		TTL:         10 * time.Minute,
		MaxAttempts: 5,
		MaxResends:  3,
	})
	if err != nil {
		return nil, err
	}

	return &ForgotPasswordResponse{
		Message: "OTP has been sent to your phone",
		OTPID:   result.OTPID,
	}, nil
}

func (s *Service) ForgotPassword(ctx context.Context, req ForgotPasswordRequest, deviceID string) (*ForgotPasswordResponse, error) {
	return s.issueForgotPasswordOTP(ctx, req, deviceID)
}

func (s *Service) VerifyForgotPasswordOTP(ctx context.Context, deviceID string, req VerifyForgotPasswordOTPRequest) (*VerifyForgotPasswordOTPResponse, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, errors.New("device id is required")
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

	result, err := s.otpManager.Verify(ctx, authotp.VerifyOTPInput{
		Purpose: authotp.PurposePasswordReset,
		OTPID:   strings.TrimSpace(req.OTPID),
		Code:    strings.TrimSpace(req.OTPCode),
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("otp verification failed")
	}

	return &VerifyForgotPasswordOTPResponse{
		Message:        "OTP verified successfully",
		VerificationID: result.VerificationID,
	}, nil
}

func (s *Service) ResetPassword(ctx context.Context, req ResetPasswordRequest, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("device id is required")
	}

	if strings.TrimSpace(req.VerificationID) == "" {
		return errors.New("verification id is required")
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

	hashedPassword, err := HashPassword(req.NewPassword)
	if err != nil {
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

		if err := serviceRepo.UpdateUserPassword(ctx, user.ID, hashedPassword); err != nil {
			return err
		}

		return nil
	})
}

func (s *Service) ResendForgotPasswordOTP(ctx context.Context, req ForgotPasswordRequest, deviceID string) (*ResendForgotPasswordOTPResponse, error) {
	resp, err := s.issueForgotPasswordOTP(ctx, req, deviceID)
	if err != nil {
		return nil, err
	}
	return &ResendForgotPasswordOTPResponse{
		Message: "OTP resent successfully",
		OTPID:   resp.OTPID,
	}, nil
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
