package wallet

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	maxPinAttempts  = 5
	pinLockDuration = 30 * time.Minute
)

func (s *Service) verifyTransactionPin(ctx context.Context, mobileUserID, pin string) error {
	user, err := s.repo.GetUserForPinVerification(ctx, mobileUserID)
	if err != nil {
		return fmt.Errorf("failed to fetch user: %w", err)
	}

	if user.TransactionPinLockedUntil != nil && user.TransactionPinLockedUntil.After(time.Now().UTC()) {
		return ErrTransactionPinLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(pin)); err != nil {
		newAttempts := user.FailedTransactionPinAttempts + 1
		if newAttempts >= maxPinAttempts {
			_ = s.repo.LockTransactionPin(ctx, mobileUserID, time.Now().UTC().Add(pinLockDuration))
			return ErrTransactionPinLocked
		}
		_ = s.repo.IncrementFailedPinAttempts(ctx, mobileUserID)
		return fmt.Errorf("%w: you have %d attempt(s) left", ErrWrongTransactionPin, maxPinAttempts-newAttempts)
	}

	_ = s.repo.ResetPinAttempts(ctx, mobileUserID)
	return nil
}
