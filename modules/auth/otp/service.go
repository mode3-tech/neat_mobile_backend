package otp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"neat_mobile_app_backend/internal/notify"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/auth/transaction"
	"neat_mobile_app_backend/modules/auth/verification"
	"neat_mobile_app_backend/providers/jwt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OTPService struct {
	repo         OTPRepository
	verification *verification.VerificationRepo
	tx           *transaction.TransactionRepository
	sms          notify.SMSSender
	email        notify.EmailSender
	jwt          *jwt.Signer
	pepper       string
}

func NewOTPService(repo OTPRepository, verification *verification.VerificationRepo, tx *transaction.TransactionRepository, sms notify.SMSSender, email notify.EmailSender, JWTSigner *jwt.Signer, pepper string) *OTPService {
	return &OTPService{repo: repo, verification: verification, tx: tx, sms: sms, email: email, jwt: JWTSigner, pepper: pepper}
}

func (o *OTPService) SendOTP(ctx context.Context, purpose Purpose, destination string, channel Channel) error {
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

	hashedOTP, err := HashOTP(o.pepper, purpose, normalizedDestination, generatedOTP)
	if err != nil {
		return errors.New("unable to hash OTP")
	}

	// TODO: use an outbox/job queue so network I/O is not inside the DB tx.
	return o.repo.WithTx(ctx, func(r *OTPRepository) error {
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
			if err := o.sms.Send(ctx, normalizedDestination, string(fmt.Sprintf("Your verification code is %s. It expires in 5 minutes. Do not share this code with anyone.", generatedOTP))); err != nil {
				return err
			}
		case ChannelEmail:
			if err := o.email.Send(ctx, normalizedDestination, "Your One Time Password (OTP)", generatedOTP); err != nil {
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
func (o *OTPService) VerifyOTP(ctx context.Context, otpCode string, destination string, channel Channel, purpose Purpose) (*VerifyOTPResponse, error) {
	now := time.Now().UTC()

	normalizedDestination, err := NormalizeDestination(destination, channel)
	if err != nil {
		return nil, errors.New("invalid otp")
	}

	var resp VerifyOTPResponse
	var verifyErr error

	err = o.tx.WithTx(ctx, func(tx *gorm.DB) error {
		r := NewOTPRepository(tx)
		verificationRepo := verification.NewVerification(tx)

		active, err := r.GetActiveOTP(ctx, normalizedDestination, purpose)
		if err != nil {
			return err
		}
		if active == nil {
			if err = o.addFailedVerification(ctx, verificationRepo, channel, normalizedDestination, "no active otp found"); err != nil {
				return err
			}
			return errors.New("invalid otp")
		}
		if active.AttemptCount >= active.MaxAttempts {
			if err = o.addFailedVerification(ctx, verificationRepo, channel, normalizedDestination, "too many failed attempts"); err != nil {
				return err
			}
			return errors.New("invalid otp")
		}

		hashed, err := HashOTP(o.pepper, purpose, normalizedDestination, otpCode)
		if err != nil {
			return errors.New("invalid otp")
		}

		if !HashEqualHex(hashed, active.OTPHash) {
			if err = r.IncrementAttempt(ctx, active.ID); err != nil {
				return err
			}
			if err = o.addFailedVerification(ctx, verificationRepo, channel, normalizedDestination, "incorrect otp"); err != nil {
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
			Message:        "OTP verification was successful",
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

func (o *OTPService) addFailedVerification(ctx context.Context, repo *verification.VerificationRepo, channel Channel, destination string, reason string) error {
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
