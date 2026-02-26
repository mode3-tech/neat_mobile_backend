package otp

import (
	"context"
	"errors"
	"neat_mobile_app_backend/internal/notify"
	"neat_mobile_app_backend/providers/jwt"
	"time"

	"github.com/google/uuid"
)

type OTPService struct {
	repo   OTPRepository
	sms    notify.SMSSender
	email  notify.EmailSender
	jwt    *jwt.Signer
	pepper string
}

func NewOTPService(repo OTPRepository, sms notify.SMSSender, email notify.EmailSender, JWTSigner *jwt.Signer, pepper string) *OTPService {
	return &OTPService{repo: repo, sms: sms, email: email, jwt: JWTSigner, pepper: pepper}
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
			if err := o.sms.Send(ctx, normalizedDestination, "Your OTP is: "); err != nil {
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
func (o *OTPService) VerifyOTP(ctx context.Context, otpCode string, destination string, channel Channel, purpose Purpose) error {
	now := time.Now().UTC()

	normalizedDestination, err := NormalizeDestination(destination, channel)
	if err != nil {
		return errors.New("invalid otp")
	}

	return o.repo.WithTx(ctx, func(r *OTPRepository) error {
		active, err := r.GetActiveOTP(ctx, normalizedDestination, purpose)
		if err != nil {
			return err
		}
		if active == nil {
			return errors.New("invalid otp")
		}
		if active.AttemptCount >= active.MaxAttempts {
			return errors.New("invalid otp")
		}

		hashed, err := HashOTP(o.pepper, purpose, normalizedDestination, otpCode)
		if err != nil {
			return errors.New("invalid otp")
		}

		if !HashEqualHex(hashed, active.OTPHash) {
			if err := r.IncrementAttempt(ctx, active.ID); err != nil {
				return err
			}
			return errors.New("invalid otp")
		}

		return r.ConsumeOTP(ctx, active.ID, now)
	})
}
