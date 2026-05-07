package pinverifier

import (
	"context"
	"errors"
	"fmt"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/models"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	maxPinAttempts  = 5
	pinLockDuration = 30 * time.Minute
)

type PinRepository interface {
	GetUserForPinVerification(ctx context.Context, userID string) (*models.User, error)
	IncrementFailedPinAttempts(ctx context.Context, userID string) error
	LockTransactionPin(ctx context.Context, userID string, until time.Time) error
	ResetPinAttempts(ctx context.Context, userID string) error
}

type Verifier struct {
	repo PinRepository
}

func New(repo PinRepository) *Verifier {
	return &Verifier{repo: repo}
}

func (v *Verifier) Verify(ctx context.Context, mobileUserID, pin string) error {
	user, err := v.repo.GetUserForPinVerification(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appErr.ErrUnauthorized
		}
		return fmt.Errorf("failed to retrieve user for pin verification: %w", err)
	}

	if user.TransactionPinLockedUntil != nil && user.TransactionPinLockedUntil.After(time.Now().UTC()) {
		return appErr.ErrTransactionPinLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(pin)); err != nil {
		newAttempts := user.FailedTransactionPinAttempts + 1
		if newAttempts >= maxPinAttempts {
			_ = v.repo.LockTransactionPin(ctx, mobileUserID, time.Now().UTC().Add(pinLockDuration))
			return appErr.ErrTransactionPinLocked
		}
		_ = v.repo.IncrementFailedPinAttempts(ctx, mobileUserID)
		return fmt.Errorf("%w: you have %d attempt(s) left", appErr.ErrIncorrectTransactionPin, maxPinAttempts-newAttempts)
	}

	_ = v.repo.ResetPinAttempts(ctx, mobileUserID)
	return nil
}
