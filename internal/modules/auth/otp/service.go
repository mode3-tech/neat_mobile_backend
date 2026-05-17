package otp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/database/tx"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/modules/auth/verification"
	"neat_mobile_app_backend/internal/notify"
	"neat_mobile_app_backend/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repo         *Repository
	verification *verification.VerificationRepo
	tx           *tx.Transactor
	sms          notify.SMSSender
	email        notify.EmailSender
	pepper       string
	appName      string
}

func NewOTPService(repo *Repository, verification *verification.VerificationRepo, tx *tx.Transactor, sms notify.SMSSender, email notify.EmailSender, pepper, appName string) *Service {
	return &Service{repo: repo, verification: verification, tx: tx, sms: sms, email: email, pepper: pepper, appName: appName}
}

func NewOTPManager(repo *Repository, verification *verification.VerificationRepo, tx *tx.Transactor, sms notify.SMSSender, email notify.EmailSender, pepper, appName string) OTPManager {
	return NewOTPService(repo, verification, tx, sms, email, pepper, appName)
}

func (s *Service) Issue(ctx context.Context, in IssueOTPInput) (*IssueOTPResult, error) {
	log.Printf("[otp.Issue] start: purpose=%s channel=%s verificationID=%q hasDestination=%v", in.Purpose, in.Channel, in.VerificationID, in.Destination != "")
	now := time.Now().UTC()

	var normalizeDestination string
	var err error

	if in.VerificationID != "" {
		row, err := s.repo.GetVerificationRow(ctx, in.VerificationID)
		if err != nil {
			log.Printf("[otp.Issue] failed to fetch verification row: verificationID=%s err=%v", in.VerificationID, err)
			return nil, err
		}
		if row == nil {
			log.Printf("[otp.Issue] verification record not found: verificationID=%s", in.VerificationID)
			return nil, appErr.ErrInvalidVerificationID
		}

		switch in.Channel {
		case ChannelSMS:
			if row.VerifiedPhone == nil || *row.VerifiedPhone == "" {
				log.Printf("[otp.Issue] no verified phone number found in verification record: verificationID=%s", in.VerificationID)
				return nil, appErr.ErrInvalidVerificationID
			}
			normalizeDestination, err = NormalizeDestination(*row.VerifiedPhone, in.Channel)
		case ChannelEmail:
			if row.VerifiedEmail == nil || *row.VerifiedEmail == "" {
				log.Printf("[otp.Issue] no verified email found in verification record: verificationID=%s", in.VerificationID)
				return nil, appErr.ErrInvalidVerificationID
			}
			normalizeDestination, err = NormalizeDestination(*row.VerifiedEmail, in.Channel)
		default:
			log.Printf("[otp.Issue] unsupported channel: channel=%s", in.Channel)
			return nil, appErr.ErrInvalidChannel
		}
		if err != nil {
			log.Printf("[otp.Issue] failed to normalize destination: channel=%s err=%v", in.Channel, err)
			return nil, err
		}
	} else if in.Destination != "" {
		normalizeDestination, err = NormalizeDestination(in.Destination, in.Channel)
		if err != nil {
			log.Printf("[otp.Issue] failed to normalize destination: channel=%s err=%v", in.Channel, err)
			return nil, err
		}
	} else {
		log.Printf("[otp.Issue] neither verificationID nor destination provided")
		return nil, appErr.ErrInvalidVerificationID
	}

	ttl := in.TTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	maxAttempts := in.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	maxResends := in.MaxResends
	if maxResends <= 0 {
		maxResends = 3
	}

	const cooldown = 30 * time.Second

	code, err := Generate6DigitOTP()
	if err != nil {
		log.Printf("[otp.Issue] failed to generate OTP: err=%v", err)
		return nil, appErr.ErrUnableToGenerateOTP
	}

	hashedOTP, err := HashOTP(s.pepper, in.Purpose, normalizeDestination, code)
	if err != nil {
		log.Printf("[otp.Issue] failed to hash OTP: err=%v", err)
		return nil, appErr.ErrUnableToHashOTP
	}

	var result IssueOTPResult

	err = s.repo.WithTx(ctx, func(r *Repository) error {
		active, err := r.GetActiveOTP(ctx, normalizeDestination, in.Purpose)
		if err != nil {
			log.Printf("[otp.Issue] failed to fetch active OTP: purpose=%s err=%v", in.Purpose, err)
			return err
		}

		if active != nil {
			if active.NextSendAt != nil && now.Before(*active.NextSendAt) {
				log.Printf("[otp.Issue] rate limited — cooldown not elapsed: otpID=%s nextSendAt=%s", active.ID, active.NextSendAt.Format(time.RFC3339))
				return appErr.ErrTooManyRequests
			}
			if active.ResendCount >= active.MaxResends {
				log.Printf("[otp.Issue] rate limited — max resends reached: otpID=%s resendCount=%d maxResends=%d", active.ID, active.ResendCount, active.MaxResends)
				return appErr.ErrTooManyRequests
			}
		}

		var smsMsg string
		switch in.Purpose {
		case PurposePasswordReset:
			smsMsg = fmt.Sprintf("%s: Your password reset code is %s. Expires in %d minutes. If you didn`t request a password reset, contact support immediately.", s.appName, code, int(ttl.Minutes()))
		case PurposeLogin:
			smsMsg = fmt.Sprintf("%s: Login verification code: %s. Expires in %d min. If this wasn`t you, secure your account immediately.", s.appName, code, int(ttl.Minutes()))
		default:
			smsMsg = fmt.Sprintf("%s: Your verification code is %s. It expires in %d minutes. Do not share this code.", s.appName, code, int(ttl.Minutes()))
		}

		switch in.Channel {
		case ChannelSMS:
			if err := s.sms.Send(ctx, normalizeDestination, smsMsg); err != nil {
				log.Printf("[otp.Issue] failed to send SMS: purpose=%s err=%v", in.Purpose, err)
				return err
			}
			log.Printf("[otp.Issue] SMS sent: purpose=%s", in.Purpose)
		case ChannelEmail:
			subject := "Your One Time Password (OTP)"
			if in.Purpose == PurposePasswordReset {
				subject = "Your Password Reset OTP"
			}
			if err := s.email.Send(ctx, normalizeDestination, subject, code); err != nil {
				log.Printf("[otp.Issue] failed to send email: purpose=%s err=%v", in.Purpose, err)
				return err
			}
			log.Printf("[otp.Issue] email sent: purpose=%s", in.Purpose)
		default:
			log.Printf("[otp.Issue] unsupported channel: channel=%s", in.Channel)
			return appErr.ErrInvalidChannel
		}

		expiresAt := now.Add(ttl)
		nextSendAt := now.Add(cooldown)

		if active == nil {
			otpRow := &OTPModel{
				ID:           uuid.NewString(),
				UserID:       in.UserID,
				Purpose:      in.Purpose,
				Channel:      in.Channel,
				Destination:  normalizeDestination,
				OTPHash:      hashedOTP,
				ExpiresAt:    expiresAt,
				NextSendAt:   &nextSendAt,
				ResendCount:  0,
				MaxResends:   maxResends,
				AttemptCount: 0,
				MaxAttempts:  maxAttempts,
				IssuedAt:     now,
			}
			if err := r.CreateOTP(ctx, otpRow); err != nil {
				log.Printf("[otp.Issue] failed to create OTP record: err=%v", err)
				return err
			}
			log.Printf("[otp.Issue] OTP created: otpID=%s purpose=%s channel=%s expiresAt=%s", otpRow.ID, in.Purpose, in.Channel, expiresAt.Format(time.RFC3339))
			result = IssueOTPResult{
				OTPID:      otpRow.ID,
				ExpiresAt:  expiresAt,
				NextSendAt: &nextSendAt,
			}
			return nil
		}

		if err := r.UpdateForResend(ctx, active.ID, hashedOTP, expiresAt, nextSendAt); err != nil {
			log.Printf("[otp.Issue] failed to update OTP for resend: otpID=%s err=%v", active.ID, err)
			return err
		}
		log.Printf("[otp.Issue] OTP resent: otpID=%s purpose=%s channel=%s resendCount=%d expiresAt=%s", active.ID, in.Purpose, in.Channel, active.ResendCount+1, expiresAt.Format(time.RFC3339))
		result = IssueOTPResult{
			OTPID:      active.ID,
			ExpiresAt:  expiresAt,
			NextSendAt: &nextSendAt,
		}
		return nil
	})

	if err != nil {
		log.Printf("[otp.Issue] transaction failed: purpose=%s channel=%s err=%v", in.Purpose, in.Channel, err)
		return nil, err
	}

	log.Printf("[otp.Issue] done: otpID=%s", result.OTPID)
	return &result, nil
}

func (s *Service) Verify(ctx context.Context, in VerifyOTPInput) (*VerifyOTPResult, error) {
	log.Printf("[otp.Verify] start: verificationID=%q otpID=%q purpose=%s channel=%s", in.VerificationID, in.OTPID, in.Purpose, in.Channel)
	now := time.Now().UTC()

	var result VerifyOTPResult
	var verifyErr error

	err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		r := NewRepository(txDB)
		verificationRepo := verification.NewVerification(txDB)

		var active *OTPModel
		var err error

		otpID := strings.TrimSpace(in.OTPID)

		if otpID != "" && in.VerificationID == "" {
			// Caller provided an OTP ID directly (e.g. password change / forgot password).
			// Look up the OTP record first; its Destination carries the channel address.
			active, err = r.GetActiveOTPByID(ctx, otpID, in.Purpose)
			if err != nil {
				log.Printf("[otp.Verify] failed to fetch active OTP by ID: otpID=%s err=%v", otpID, err)
				return err
			}
			if active == nil {
				log.Printf("[otp.Verify] no active OTP found by ID: otpID=%s purpose=%s", otpID, in.Purpose)
				return appErr.ErrInvalidOTP
			}
		} else {
			// Caller provided a verification_id + channel — derive destination from that record.
			row, err := s.repo.GetVerificationRow(ctx, in.VerificationID)
			if err != nil {
				log.Printf("[otp.Verify] failed to fetch verification row: verificationID=%s err=%v", in.VerificationID, err)
				return err
			}
			if row == nil {
				log.Printf("[otp.Verify] verification record not found: verificationID=%s", in.VerificationID)
				return appErr.ErrInvalidVerificationID
			}

			var destination string
			switch in.Channel {
			case ChannelSMS:
				if row.VerifiedPhone == nil || *row.VerifiedPhone == "" {
					log.Printf("[otp.Verify] no verified phone in verification record: verificationID=%s", in.VerificationID)
					return appErr.ErrInvalidVerificationID
				}
				destination = *row.VerifiedPhone
			case ChannelEmail:
				if row.VerifiedEmail == nil || *row.VerifiedEmail == "" {
					log.Printf("[otp.Verify] no verified email in verification record: verificationID=%s", in.VerificationID)
					return appErr.ErrInvalidVerificationID
				}
				destination = *row.VerifiedEmail
			default:
				log.Printf("[otp.Verify] unsupported channel: channel=%s", in.Channel)
				return appErr.ErrInvalidChannel
			}

			normalizedDestination, normErr := NormalizeDestination(destination, in.Channel)
			if normErr != nil {
				if in.Channel == ChannelSMS {
					log.Printf("[otp.Verify] failed to normalize phone number: err=%v", normErr)
					return appErr.ErrInvalidPhone
				}
				log.Printf("[otp.Verify] failed to normalize email: err=%v", normErr)
				return appErr.ErrInvalidEmail
			}

			if otpID != "" {
				active, err = r.GetActiveOTPByID(ctx, otpID, in.Purpose)
			} else {
				active, err = r.GetActiveOTP(ctx, normalizedDestination, in.Purpose)
			}
			if err != nil {
				log.Printf("[otp.Verify] failed to fetch active OTP: purpose=%s err=%v", in.Purpose, err)
				return err
			}
			if active == nil {
				log.Printf("[otp.Verify] no active OTP found: purpose=%s channel=%s", in.Purpose, in.Channel)
				return appErr.ErrInvalidOTP
			}
		}

		maxAttempts := active.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		if active.AttemptCount >= maxAttempts {
			log.Printf("[otp.Verify] max attempts reached: otpID=%s attemptCount=%d maxAttempts=%d", active.ID, active.AttemptCount, maxAttempts)
			return appErr.ErrInvalidOTP
		}

		hashed, err := HashOTP(s.pepper, in.Purpose, active.Destination, strings.TrimSpace(in.Code))
		if err != nil {
			log.Printf("[otp.Verify] failed to hash submitted code: otpID=%s err=%v", active.ID, err)
			return appErr.ErrInvalidOTP
		}

		if !HashEqualHex(hashed, active.OTPHash) {
			if err = r.IncrementAttempt(ctx, active.ID); err != nil {
				log.Printf("[otp.Verify] failed to increment attempt count: otpID=%s err=%v", active.ID, err)
				return err
			}
			log.Printf("[otp.Verify] incorrect OTP code: otpID=%s attemptCount=%d", active.ID, active.AttemptCount+1)
			verifyErr = appErr.ErrInvalidOTP
			return nil
		}

		if err = r.ConsumeOTP(ctx, active.ID, now); err != nil {
			log.Printf("[otp.Verify] failed to consume OTP: otpID=%s err=%v", active.ID, err)
			return err
		}
		record, err := newVerifiedVerificationRecord(active.Channel, active.Destination, now)
		if err != nil {
			log.Printf("[otp.Verify] failed to build verification record: otpID=%s err=%v", active.ID, err)
			return err
		}

		if err := verificationRepo.AddVerification(ctx, record); err != nil {
			log.Printf("[otp.Verify] failed to persist verification record: otpID=%s err=%v", active.ID, err)
			return err
		}

		log.Printf("[otp.Verify] OTP verified: otpID=%s verificationID=%s userID=%s", active.ID, record.ID, active.UserID)
		result = VerifyOTPResult{
			OTPID:          active.ID,
			UserID:         active.UserID,
			VerifiedAt:     now,
			VerificationID: record.ID,
		}

		return nil
	})

	if err != nil {
		log.Printf("[otp.Verify] transaction failed: verificationID=%q otpID=%q err=%v", in.VerificationID, in.OTPID, err)
		return nil, err
	}

	if verifyErr != nil {
		return nil, verifyErr
	}

	log.Printf("[otp.Verify] done: otpID=%s verificationID=%s", result.OTPID, result.VerificationID)
	return &result, nil
}

func (s *Service) SendOTP(ctx context.Context, purpose Purpose, destination string, channel Channel) error {
	log.Printf("[otp.SendOTP] start: purpose=%s channel=%s", purpose, channel)
	now := time.Now().UTC()

	normalizedDestination, err := NormalizeDestination(destination, channel)
	if err != nil {
		log.Printf("[otp.SendOTP] failed to normalize destination: channel=%s err=%v", channel, err)
		return err
	}

	const ttl = 10 * time.Minute
	const cooldown = 30 * time.Second

	generatedOTP, err := Generate6DigitOTP()
	if err != nil {
		log.Printf("[otp.SendOTP] failed to generate OTP: err=%v", err)
		return appErr.ErrUnableToGenerateOTP
	}

	hashedOTP, err := HashOTP(s.pepper, purpose, normalizedDestination, generatedOTP)
	if err != nil {
		log.Printf("[otp.SendOTP] failed to hash OTP: err=%v", err)
		return appErr.ErrUnableToHashOTP
	}

	// TODO: use an outbox/job queue so network I/O is not inside the DB tx.
	return s.repo.WithTx(ctx, func(r *Repository) error {
		active, err := r.GetActiveOTP(ctx, normalizedDestination, purpose)
		if err != nil {
			log.Printf("[otp.SendOTP] failed to fetch active OTP: purpose=%s err=%v", purpose, err)
			return err
		}

		if active != nil {
			if active.NextSendAt != nil && now.Before(*active.NextSendAt) {
				log.Printf("[otp.SendOTP] rate limited — cooldown not elapsed: otpID=%s nextSendAt=%s", active.ID, active.NextSendAt.Format(time.RFC3339))
				return appErr.ErrTooManyRequests
			}
			if active.ResendCount >= active.MaxResends {
				log.Printf("[otp.SendOTP] rate limited — max resends reached: otpID=%s resendCount=%d maxResends=%d", active.ID, active.ResendCount, active.MaxResends)
				return appErr.ErrTooManyRequests
			}
		}

		switch channel {
		case ChannelSMS:
			if err := s.sms.Send(ctx, normalizedDestination, fmt.Sprintf("Your verification code is %s. It expires in 5 minutes. Do not share this code with anyone.", generatedOTP)); err != nil {
				log.Printf("[otp.SendOTP] failed to send SMS: purpose=%s err=%v", purpose, err)
				return err
			}
			log.Printf("[otp.SendOTP] SMS sent: purpose=%s", purpose)
		case ChannelEmail:
			if err := s.email.Send(ctx, normalizedDestination, "Your One Time Password (OTP)", generatedOTP); err != nil {
				log.Printf("[otp.SendOTP] failed to send email: purpose=%s err=%v", purpose, err)
				return err
			}
			log.Printf("[otp.SendOTP] email sent: purpose=%s", purpose)
		default:
			log.Printf("[otp.SendOTP] unsupported channel: channel=%s", channel)
			return appErr.ErrInvalidChannel
		}

		expiresAt := now.Add(ttl)
		nextSendAt := now.Add(cooldown)

		if active == nil {
			otpRow := &OTPModel{
				ID:           uuid.NewString(),
				Purpose:      purpose,
				Channel:      channel,
				Destination:  normalizedDestination,
				OTPHash:      hashedOTP,
				ExpiresAt:    expiresAt,
				NextSendAt:   &nextSendAt,
				ResendCount:  0,
				MaxResends:   3,
				AttemptCount: 0,
				MaxAttempts:  5,
				IssuedAt:     now,
			}
			if err := r.CreateOTP(ctx, otpRow); err != nil {
				log.Printf("[otp.SendOTP] failed to create OTP record: err=%v", err)
				return err
			}
			log.Printf("[otp.SendOTP] OTP created: otpID=%s purpose=%s channel=%s expiresAt=%s", otpRow.ID, purpose, channel, expiresAt.Format(time.RFC3339))
			return nil
		}

		if err := r.UpdateForResend(ctx, active.ID, hashedOTP, expiresAt, nextSendAt); err != nil {
			log.Printf("[otp.SendOTP] failed to update OTP for resend: otpID=%s err=%v", active.ID, err)
			return err
		}
		log.Printf("[otp.SendOTP] OTP resent: otpID=%s purpose=%s channel=%s resendCount=%d", active.ID, purpose, channel, active.ResendCount+1)
		return nil
	})
}

// Note: verify now takes channel so normalization is deterministic.
func (s *Service) VerifyOTP(ctx context.Context, otpCode string, destination string, channel Channel, purpose Purpose) (*VerifyOTPResponse, error) {
	log.Printf("[otp.VerifyOTP] start: purpose=%s channel=%s", purpose, channel)
	now := time.Now().UTC()

	normalizedDestination, err := NormalizeDestination(destination, channel)
	if err != nil {
		log.Printf("[otp.VerifyOTP] failed to normalize destination: channel=%s err=%v", channel, err)
		return nil, appErr.ErrInvalidOTP
	}

	var resp VerifyOTPResponse
	var verifyErr error

	err = s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		r := NewRepository(tx)
		verificationRepo := verification.NewVerification(tx)

		active, err := r.GetActiveOTP(ctx, normalizedDestination, purpose)
		if err != nil {
			log.Printf("[otp.VerifyOTP] failed to fetch active OTP: purpose=%s err=%v", purpose, err)
			return err
		}
		if active == nil {
			log.Printf("[otp.VerifyOTP] no active OTP found: purpose=%s channel=%s", purpose, channel)
			if err = s.addFailedVerification(ctx, verificationRepo, channel, normalizedDestination, "no active otp found"); err != nil {
				return err
			}
			return appErr.ErrInvalidOTP
		}
		if active.AttemptCount >= active.MaxAttempts {
			log.Printf("[otp.VerifyOTP] max attempts reached: otpID=%s attemptCount=%d maxAttempts=%d", active.ID, active.AttemptCount, active.MaxAttempts)
			if err = s.addFailedVerification(ctx, verificationRepo, channel, normalizedDestination, "too many failed attempts"); err != nil {
				return err
			}
			return appErr.ErrInvalidOTP
		}

		hashed, err := HashOTP(s.pepper, purpose, normalizedDestination, otpCode)
		if err != nil {
			log.Printf("[otp.VerifyOTP] failed to hash submitted code: otpID=%s err=%v", active.ID, err)
			return appErr.ErrInvalidOTP
		}

		if !HashEqualHex(hashed, active.OTPHash) {
			if err = r.IncrementAttempt(ctx, active.ID); err != nil {
				log.Printf("[otp.VerifyOTP] failed to increment attempt count: otpID=%s err=%v", active.ID, err)
				return err
			}
			if err = s.addFailedVerification(ctx, verificationRepo, channel, normalizedDestination, "incorrect otp"); err != nil {
				return err
			}
			log.Printf("[otp.VerifyOTP] incorrect OTP code: otpID=%s attemptCount=%d", active.ID, active.AttemptCount+1)
			verifyErr = appErr.ErrInvalidOTP
			return nil
		}

		if err = r.ConsumeOTP(ctx, active.ID, now); err != nil {
			log.Printf("[otp.VerifyOTP] failed to consume OTP: otpID=%s err=%v", active.ID, err)
			return err
		}
		record, err := newVerifiedVerificationRecord(channel, normalizedDestination, now)
		if err != nil {
			log.Printf("[otp.VerifyOTP] failed to build verification record: otpID=%s err=%v", active.ID, err)
			return err
		}

		if err := verificationRepo.AddVerification(ctx, record); err != nil {
			log.Printf("[otp.VerifyOTP] failed to persist verification record: otpID=%s err=%v", active.ID, err)
			return err
		}

		log.Printf("[otp.VerifyOTP] OTP verified: otpID=%s verificationID=%s", active.ID, record.ID)
		resp = VerifyOTPResponse{
			VerificationID: record.ID,
		}

		return nil
	})

	if err != nil {
		log.Printf("[otp.VerifyOTP] transaction failed: purpose=%s channel=%s err=%v", purpose, channel, err)
		return nil, err
	}

	if verifyErr != nil {
		return nil, verifyErr
	}

	log.Printf("[otp.VerifyOTP] done: verificationID=%s", resp.VerificationID)
	return &resp, nil
}

func (s *Service) addFailedVerification(ctx context.Context, repo *verification.VerificationRepo, channel Channel, destination string, reason string) error {
	record, err := newFailedVerificationRecord(channel, destination, reason)
	if err != nil {
		return err
	}

	return repo.AddVerification(ctx, record)
}

func newFailedVerificationRecord(channel Channel, destination string, reason string) (*models.VerificationRecord, error) {
	record, err := newVerificationRecord(channel, destination)
	if err != nil {
		return nil, err
	}

	record.Status = models.VerificationStatusFailed
	record.FailureReason = &reason

	return record, nil
}

func newVerifiedVerificationRecord(channel Channel, destination string, verifiedAt time.Time) (*models.VerificationRecord, error) {
	record, err := newVerificationRecord(channel, destination)
	if err != nil {
		return nil, err
	}

	expiresAt := verifiedAt.Add(15 * time.Minute)
	record.Status = models.VerificationStatusVerified
	record.VerifiedAt = &verifiedAt
	record.ExpiresAt = &expiresAt

	switch channel {
	case ChannelEmail:
		record.VerifiedEmail = &destination
	case ChannelSMS:
		record.VerifiedPhone = &destination
	}

	return record, nil
}

func newVerificationRecord(channel Channel, destination string) (*models.VerificationRecord, error) {
	destination = strings.TrimSpace(destination)
	subjectHashBytes := sha256.Sum256([]byte(destination))
	subjectHash := hex.EncodeToString(subjectHashBytes[:])

	record := &models.VerificationRecord{
		ID:          uuid.NewString(),
		SubjectHash: subjectHash,
	}

	switch channel {
	case ChannelEmail:
		record.Type = models.VerificationTypeEmail
	case ChannelSMS:
		record.Type = models.VerificationTypePhone
		record.Provider = string(ProviderTermii)
	default:
		return nil, appErr.ErrInvalidChannel
	}

	return record, nil
}
