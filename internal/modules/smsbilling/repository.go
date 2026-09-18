package smsbilling

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BAASDebitor is the subset of the Providus BaaS client needed to settle a
// collected SMS charge as a real wallet debit. Providus customer wallets are
// sub-accounts of the platform's own merchant/aggregator account, so this
// debit is what actually moves the SMS fee into the merchant's settlement
// balance - there is no separate "merchant wallet" customer id to credit.
//
// Declared with only primitive/stdlib types (not providers/baas's response
// type) so this package doesn't import providers/baas - that would form the
// cycle smsbilling -> baas -> auth -> wallet -> smsbilling. See
// internal/adapters/providus for the concrete adapter wired in by router.go.
type BAASDebitor interface {
	DebitCustomer(ctx context.Context, amount int64, customerID, referenceID string, metadata interface{}) error
}

type Repository struct {
	db   *gorm.DB
	baas BAASDebitor
}

func NewRepository(db *gorm.DB, baas BAASDebitor) *Repository {
	return &Repository{db: db, baas: baas}
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

// markRetryOrAbandon bumps a charge's attempt counter and either leaves it
// pending for a later retry or, once maxAttempts is reached, marks it
// abandoned. Shared by the local-insufficient-funds and real-debit-failed
// branches of AttemptCollect below - both are "try again later" outcomes.
func markRetryOrAbandon(tx *gorm.DB, charge *SMSCharge, now time.Time, maxAttempts int) (CollectResult, error) {
	attempts := charge.Attempts + 1
	updates := map[string]any{
		"attempts":        attempts,
		"last_attempt_at": now,
	}
	result := CollectResultInsufficient
	if attempts >= maxAttempts {
		updates["status"] = SMSChargeStatusAbandoned
		result = CollectResultAbandoned
	}
	return result, tx.Model(&SMSCharge{}).Where("id = ?", charge.ID).Updates(updates).Error
}

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
			var rerr error
			result, rerr = markRetryOrAbandon(tx, &charge, now, maxAttempts)
			return rerr
		}

		// Settle with a real Providus debit before touching the local
		// ledger, so a charge is only ever marked successful once the money
		// has actually moved out of the customer's real wallet (and into
		// the merchant's settlement balance). A failed real debit is
		// retried exactly like local insufficient funds - the existing
		// sweep and credit-triggered recovery paths pick it back up.
		amountNaira := charge.AmountKobo / 100
		metadata := map[string]any{
			"charge_id": charge.ID,
			"purpose":   charge.Purpose,
		}
		if derr := r.baas.DebitCustomer(ctx, amountNaira, wallet.WalletCustomerID, charge.Reference, metadata); derr != nil {
			log.Printf("smsbilling: providus debit failed: charge=%s customer=%s err=%v", charge.ID, wallet.WalletCustomerID, derr)
			var rerr error
			result, rerr = markRetryOrAbandon(tx, &charge, now, maxAttempts)
			return rerr
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
