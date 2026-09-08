package smsbilling

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateCharge(ctx context.Context, mobileUserID, phone, purpose string, units int, unitPriceKobo int64) (*SMSCharge, error) {
	charge := &SMSCharge{
		ID:            uuid.NewString(),
		MobileUserID:  mobileUserID,
		Phone:         phone,
		Purpose:       purpose,
		Units:         units,
		UnitPriceKobo: unitPriceKobo,
		AmountKobo:    unitPriceKobo * int64(units),
		Status:        SMSChargeStatusPending,
		Reference:     "sms-charge-" + uuid.NewString(),
	}
	if err := r.db.WithContext(ctx).Create(charge).Error; err != nil {
		return nil, err
	}
	return charge, nil
}

type CollectResult string

const (
	CollectResultCollected    CollectResult = "collected"
	CollectResultInsufficient CollectResult = "insufficient"
	CollectResultAbandoned    CollectResult = "abandoned"
	// CollectResultSkipped means the charge was already resolved (or claimed
	// by a concurrent attempt) by the time this call got its lock.
	CollectResultSkipped CollectResult = "skipped"
)

// AttemptCollect makes one atomic attempt to collect a pending charge: it
// locks the charge row first (serializing concurrent attempts arriving from
// the immediate post-send path, credit-triggered recovery, and the sweep, so
// a charge is never collected twice), then locks the customer's wallet row
// and either debits it and writes the wallet_transactions row in the same DB
// transaction, or bumps the attempt counter and leaves the charge pending -
// abandoning it once maxAttempts is reached rather than retrying forever.
func (r *Repository) AttemptCollect(ctx context.Context, chargeID string, maxAttempts int) (CollectResult, error) {
	result := CollectResultSkipped

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var charge SMSCharge
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", chargeID, SMSChargeStatusPending).
			First(&charge).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		now := time.Now().UTC()

		var wallet customerWallet
		werr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("mobile_user_id = ?", charge.MobileUserID).
			First(&wallet).Error
		if werr != nil && !errors.Is(werr, gorm.ErrRecordNotFound) {
			return werr
		}

		if werr != nil || wallet.AvailableBalance < charge.AmountKobo {
			attempts := charge.Attempts + 1
			updates := map[string]any{
				"attempts":        attempts,
				"last_attempt_at": now,
			}
			if attempts >= maxAttempts {
				updates["status"] = SMSChargeStatusAbandoned
				result = CollectResultAbandoned
			} else {
				result = CollectResultInsufficient
			}
			return tx.Model(&SMSCharge{}).Where("id = ?", charge.ID).Updates(updates).Error
		}

		txnID := uuid.NewString()
		txn := ledgerTransaction{
			ID:            txnID,
			MobileUserID:  charge.MobileUserID,
			WalletID:      wallet.InternalWalletID,
			Type:          "debit",
			Category:      "sms",
			Amount:        charge.AmountKobo,
			BalanceBefore: wallet.AvailableBalance,
			BalanceAfter:  wallet.AvailableBalance - charge.AmountKobo,
			Reference:     charge.Reference,
			Description:   "SMS fee",
			Status:        "successful",
			Source:        "debit",
			CreatedAt:     now,
		}
		if err := tx.Create(&txn).Error; err != nil {
			return err
		}

		if err := tx.Model(&customerWallet{}).
			Where("internal_wallet_id = ?", wallet.InternalWalletID).
			Updates(map[string]any{
				"booked_balance":    gorm.Expr("booked_balance - ?", charge.AmountKobo),
				"available_balance": gorm.Expr("available_balance - ?", charge.AmountKobo),
				"updated_at":        now,
			}).Error; err != nil {
			return err
		}

		if err := tx.Model(&SMSCharge{}).Where("id = ?", charge.ID).
			Updates(map[string]any{
				"status":          SMSChargeStatusSuccessful,
				"transaction_id":  txnID,
				"attempts":        charge.Attempts + 1,
				"last_attempt_at": now,
			}).Error; err != nil {
			return err
		}

		result = CollectResultCollected
		return nil
	})

	return result, err
}

// ListOutstandingForUser returns pending charge IDs for one user, oldest
// first - the credit-triggered recovery path.
func (r *Repository) ListOutstandingForUser(ctx context.Context, mobileUserID string, limit int) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).
		Model(&SMSCharge{}).
		Where("mobile_user_id = ? AND status = ?", mobileUserID, SMSChargeStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

// ListOutstandingCandidates returns pending charge IDs across all users that
// are due a retry - the sweep's candidate list. Mutual exclusion between
// concurrent collectors happens per-charge inside AttemptCollect, so this
// query only needs to pick candidates, not lock them.
func (r *Repository) ListOutstandingCandidates(ctx context.Context, limit int, staleAfter time.Duration, maxAttempts int) ([]string, error) {
	var ids []string
	cutoff := time.Now().UTC().Add(-staleAfter)
	err := r.db.WithContext(ctx).
		Model(&SMSCharge{}).
		Where("status = ? AND attempts < ?", SMSChargeStatusPending, maxAttempts).
		Where("last_attempt_at IS NULL OR last_attempt_at < ?", cutoff).
		Order("created_at ASC").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}
