package otp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/database/tx"
	"neat_mobile_app_backend/internal/notify"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/auth/verification"
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
}

func NewOTPService(repo *Repository, verification *verification.VerificationRepo, tx *tx.Transactor, sms notify.SMSSender, email notify.EmailSender, pepper string) *Service {
	return &Service{repo: repo, verification: verification, tx: tx, sms: sms, email: email, pepper: pepper}
}

func NewOTPManager(repo *Repository, verification *verification.VerificationRepo, tx *tx.Transactor, sms notify.SMSSender, email notify.EmailSender, pepper string) OTPManager {
	return NewOTPService(repo, verification, tx, sms, email, pepper)
}

func (s *Service) Issue(ctx context.Context, in IssueOTPInput) (*IssueOTPResult, error) {
	now := time.Now().UTC()

	var normalizeDestination string
	var err error

	if in.VerificationID != "" {
		row, err := s.repo.GetVerificationRow(ctx, in.VerificationID)
		if err != nil {
			return nil, err
		}
		if row == nil {
			log.Printf("verification record not found")
			return nil, errors.New("verification record not found")
		}

		switch in.Channel {
		case ChannelSMS:
			if row.VerifiedPhone == nil || *row.VerifiedPhone == "" {
				log.Printf("no verified phone number found in verification record")
				return nil, errors.New("no verified phone number found in verification record")
			}
			normalizeDestination, err = NormalizeDestination(*row.VerifiedPhone, in.Channel)
		case ChannelEmail:
			if row.VerifiedEmail == nil || *row.VerifiedEmail == "" {
				log.Printf("no verified email found in verification record")
				return nil, errors.New("no verified email found in verification record")
			}
			normalizeDestination, err = NormalizeDestination(*row.VerifiedEmail, in.Channel)
		default:
			return nil, errors.New("unsupported channel")
		}
		if err != nil {
			return nil, err
		}
	} else if in.Destination != "" {
		normalizeDestination, err = NormalizeDestination(in.Destination, in.Channel)
		if err != nil {
			return nil, err
		}
	} else {
		log.Printf("either verification_id or destination must be provided")
		return nil, errors.New("either verification_id or destination must be provided")
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
		return nil, errors.New("unable to generate OTP")
	}

	hashedOTP, err := HashOTP(s.pepper, in.Purpose, normalizeDestination, code)
	if err != nil {
		return nil, errors.New("unable to hash OTP")
	}

	var result IssueOTPResult

	err = s.repo.WithTx(ctx, func(r *Repository) error {
		active, err := r.GetActiveOTP(ctx, normalizeDestination, in.Purpose)
		if err != nil {
			return err
		}

		if active != nil {
			if active.NextSendAt != nil && now.Before(*active.NextSendAt) {
				return errors.New("too many requests")
			}
			if active.ResendCount >= active.MaxResends {
				return errors.New("too many requests")
			}
		}

		var smsMsg string
		switch in.Purpose {
		case PurposePasswordReset:
			smsMsg = fmt.Sprintf("Your password reset OTP is %s. It expires in %d minutes. Do not share this OTP with anyone.", code, int(ttl.Minutes()))
		case PurposeLogin:
			smsMsg = fmt.Sprintf("Your login OTP is %s. It expires in %d minutes. Do not share this code with anyone.", code, int(ttl.Minutes()))
		default:
			smsMsg = fmt.Sprintf("Your verification code is %s. It expires in %d minutes. Do not share this code with anyone.", code, int(ttl.Minutes()))
		}

		switch in.Channel {
		case ChannelSMS:
			if err := s.sms.Send(ctx, normalizeDestination, smsMsg); err != nil {
				return err
			}
		case ChannelEmail:
			subject := "Your One Time Password (OTP)"
			if in.Purpose == PurposePasswordReset {
				subject = "Your Password Reset OTP"
			}
			if err := s.email.Send(ctx, normalizeDestination, subject, code); err != nil {
				return err
			}
		default:
			return errors.New("unsupported channel")
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
				return err
			}
			result = IssueOTPResult{
				OTPID:      otpRow.ID,
				ExpiresAt:  expiresAt,
				NextSendAt: &nextSendAt,
			}
			return nil
		}

		if err := r.UpdateForResend(ctx, active.ID, hashedOTP, expiresAt, nextSendAt); err != nil {
			return err
		}
		result = IssueOTPResult{
			OTPID:      active.ID,
			ExpiresAt:  expiresAt,
			NextSendAt: &nextSendAt,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *Service) Verify(ctx context.Context, in VerifyOTPInput) (*VerifyOTPResult, error) {
	now := time.Now().UTC()

	var result VerifyOTPResult
	var verifyErr error

	err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		r := NewRepository(txDB)
		verificationRepo := verification.NewVerification(txDB)

		// Get destination from verification record
		row, err := s.repo.GetVerificationRow(ctx, in.VerificationID)
		if err != nil {
			return err
		}
		if row == nil {
			return errors.New("verification record not found")
		}

		var destination string
		switch in.Channel {
		case ChannelSMS:
			if row.VerifiedPhone == nil || *row.VerifiedPhone == "" {
				return errors.New("no verified phone number found in verification record")
			}
			destination = *row.VerifiedPhone
		case ChannelEmail:
			if row.VerifiedEmail == nil || *row.VerifiedEmail == "" {
				return errors.New("no verified email found in verification record")
			}
			destination = *row.VerifiedEmail
		default:
			return errors.New("unsupported channel")
		}

		normalizedDestination, normErr := NormalizeDestination(destination, in.Channel)
		if normErr != nil {
			return errors.New("invalid destination")
		}

		var active *OTPModel

		if strings.TrimSpace(in.OTPID) != "" {
			active, err = r.GetActiveOTPByID(ctx, strings.TrimSpace(in.OTPID), in.Purpose)
		} else {
			active, err = r.GetActiveOTP(ctx, normalizedDestination, in.Purpose)
		}

		if err != nil {
			return err
		}

		if active == nil {
			return errors.New("invalid otp")
		}

		maxAttempts := active.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		if active.AttemptCount >= maxAttempts {
			return errors.New("invalid otp")
		}

		hashed, err := HashOTP(s.pepper, in.Purpose, active.Destination, strings.TrimSpace(in.Code))
		if err != nil {
			return errors.New("invalid otp")
		}

		if !HashEqualHex(hashed, active.OTPHash) {
			if err = r.IncrementAttempt(ctx, active.ID); err != nil {
				return err
			}
			verifyErr = errors.New("invalid otp")
			return nil
		}

		if err = r.ConsumeOTP(ctx, active.ID, now); err != nil {
			return err
		}
		record, err := newVerifiedVerificationRecord(active.Channel, active.Destination, now)
		if err != nil {
			return err
		}

		if err := verificationRepo.AddVerification(ctx, record); err != nil {
			return err
		}

		result = VerifyOTPResult{
			OTPID:          active.ID,
			UserID:         active.UserID,
			VerifiedAt:     now,
			VerificationID: record.ID,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if verifyErr != nil {
		return nil, verifyErr
	}

	return &result, nil
}

func (s *Service) SendOTP(ctx context.Context, purpose Purpose, destination string, channel Channel) error {
	now := time.Now().UTC()

	normalizedDestination, err := NormalizeDestination(destination, channel)
	if err != nil {
		return err
	}

	const ttl = 10 * time.Minute
	const cooldown = 30 * time.Second

	generatedOTP, err := Generate6DigitOTP()
	if err != nil {
		return errors.New("unable to generate OTP")
	}

	hashedOTP, err := HashOTP(s.pepper, purpose, normalizedDestination, generatedOTP)
	if err != nil {
		return errors.New("unable to hash OTP")
	}

	// TODO: use an outbox/job queue so network I/s is not inside the DB tx.
	return s.repo.WithTx(ctx, func(r *Repository) error {
		active, err := r.GetActiveOTP(ctx, normalizedDestination, purpose)
		if err != nil {
			return err
		}

		if active != nil {
			if active.NextSendAt != nil && now.Before(*active.NextSendAt) {
				return errors.New("too many requests")
			}
			if active.ResendCount >= active.MaxResends {
				return errors.New("too many requests")
			}
		}

		switch channel {
		case ChannelSMS:
			if err := s.sms.Send(ctx, normalizedDestination, string(fmt.Sprintf("Your verification code is %s. It expires in 5 minutes. Do not share this code with anyone.", generatedOTP))); err != nil {
				return err
			}
		case ChannelEmail:
			if err := s.email.Send(ctx, normalizedDestination, "Your One Time Password (OTP)", generatedOTP); err != nil {
				return err
			}
		default:
			return errors.New("unsupported channel")
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
			return r.CreateOTP(ctx, otpRow)
		}

		return r.UpdateForResend(ctx, active.ID, hashedOTP, expiresAt, nextSendAt)
	})
}

// Note: verify now takes channel so normalization is deterministic.
func (s *Service) VerifyOTP(ctx context.Context, otpCode string, destination string, channel Channel, purpose Purpose) (*VerifyOTPResponse, error) {
	now := time.Now().UTC()

	normalizedDestination, err := NormalizeDestination(destination, channel)
	if err != nil {
		return nil, errors.New("invalid otp")
	}

	var resp VerifyOTPResponse
	var verifyErr error

	err = s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		r := NewRepository(tx)
		verificationRepo := verification.NewVerification(tx)

		active, err := r.GetActiveOTP(ctx, normalizedDestination, purpose)
		if err != nil {
			return err
		}
		if active == nil {
			if err = s.addFailedVerification(ctx, verificationRepo, channel, normalizedDestination, "no active otp found"); err != nil {
				return err
			}
			return errors.New("invalid otp")
		}
		if active.AttemptCount >= active.MaxAttempts {
			if err = s.addFailedVerification(ctx, verificationRepo, channel, normalizedDestination, "too many failed attempts"); err != nil {
				return err
			}
			return errors.New("invalid otp")
		}

		hashed, err := HashOTP(s.pepper, purpose, normalizedDestination, otpCode)
		if err != nil {
			return errors.New("invalid otp")
		}

		if !HashEqualHex(hashed, active.OTPHash) {
			if err = r.IncrementAttempt(ctx, active.ID); err != nil {
				return err
			}
			if err = s.addFailedVerification(ctx, verificationRepo, channel, normalizedDestination, "incorrect otp"); err != nil {
				return err
			}
			verifyErr = errors.New("invalid otp")
			return nil
		}

		if err = r.ConsumeOTP(ctx, active.ID, now); err != nil {
			return err
		}
		record, err := newVerifiedVerificationRecord(channel, normalizedDestination, now)
		if err != nil {
			return err
		}

		if err := verificationRepo.AddVerification(ctx, record); err != nil {
			return err
		}

		resp = VerifyOTPResponse{
			VerificationID: record.ID,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if verifyErr != nil {
		return nil, verifyErr
	}

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
		return nil, errors.New("unsupported channel")
	}

	return record, nil
}
