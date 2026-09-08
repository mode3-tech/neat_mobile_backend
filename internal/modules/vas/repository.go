package vas

import (
	"context"
	"errors"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/types"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) WithTx(tx *gorm.DB) *Repository {
	return &Repository{db: tx}
}

func (r *Repository) GetBalance(ctx context.Context, mobileUserID string) (*CustomerWallet, error) {
	var wallet CustomerWallet
	err := r.db.WithContext(ctx).
		Where("mobile_user_id = ?", mobileUserID).
		First(&wallet).Error
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *Repository) AddTransaction(ctx context.Context, txn *Transaction) error {
	return r.db.WithContext(ctx).Create(txn).Error
}

func (r *Repository) UpdateTransactionStatus(ctx context.Context, txID string, balanceAfter int64, status TransactionStatus) error {
	return r.db.WithContext(ctx).
		Model(&Transaction{}).
		Where("id = ?", txID).
		Updates(map[string]interface{}{
			"status":        status,
			"balance_after": balanceAfter,
		}).Error
}

func (r *Repository) UpdateTransactionProviderReference(ctx context.Context, txID, providerRef string) error {
	return r.db.WithContext(ctx).
		Model(&Transaction{}).
		Where("id = ?", txID).
		Update("provider_reference", providerRef).Error
}

func (r *Repository) UpdateTransactionMetadata(ctx context.Context, txID string, metadata map[string]any) error {
	// Wrapped in types.JSONMap so the value satisfies driver.Valuer and gets
	// marshaled to JSON for the jsonb column — a bare map[string]any has no
	// Value() method, so the driver has no defined way to encode it.
	return r.db.WithContext(ctx).
		Model(&Transaction{}).
		Where("id = ?", txID).
		Update("metadata", types.JSONMap(metadata)).Error
}

func (r *Repository) MarkWalletRefunded(ctx context.Context, txID string) error {
	return r.db.WithContext(ctx).
		Model(&Transaction{}).
		Where("id = ?", txID).
		Update("wallet_refunded", true).Error
}

func (r *Repository) MarkCashbackReleased(ctx context.Context, txID string) error {
	return r.db.WithContext(ctx).
		Model(&Transaction{}).
		Where("id = ?", txID).
		Update("cashback_released", true).Error
}

func (r *Repository) MarkTransactionRefundPending(ctx context.Context, txID string, balanceAfter int64) error {
	return r.db.WithContext(ctx).
		Model(&Transaction{}).
		Where("id = ?", txID).
		Updates(map[string]interface{}{
			"status":        TransactionStatusRefundPending,
			"balance_after": balanceAfter,
		}).Error
}

// ClaimStuckTransactions leases up to limit transactions that need
// reconciliation, across three buckets, in one SELECT ... FOR UPDATE SKIP
// LOCKED pass so concurrent sweep runs never claim the same row:
//
//   - reversal_pending: outcome unknown, needs a CheckStatus call
//   - refund_pending: outcome known-failed, refund didn't fully complete
//   - pending, stuck past stalePendingAfter with a persisted request ID:
//     crash-window orphans that need a CheckStatus call before anything else
//
// Rows already at their bucket's max-attempts threshold are excluded (left
// for manual review) and rows whose last attempt is within leaseWindow are
// excluded (being processed by another run). Claimed rows have their
// attempt counter bumped and lease timestamp refreshed before being
// returned, mirroring auth.Repository.ClaimPendingRegistrationJobs.
func (r *Repository) ClaimStuckTransactions(ctx context.Context, limit int, stalePendingAfter, leaseWindow time.Duration, maxReversalPendingAttempts, maxRefundPendingAttempts int) ([]Transaction, error) {
	if limit <= 0 {
		return []Transaction{}, nil
	}

	var txns []Transaction
	now := time.Now().UTC()
	leaseBefore := now.Add(-leaseWindow)
	stalePendingBefore := now.Add(-stalePendingAfter)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bucketQuery := `(status = ? AND reconciliation_attempts < ?)
			OR (status = ? AND reconciliation_attempts < ?)
			OR (status = ? AND created_at < ? AND vas_request_id <> ?)`

		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(bucketQuery,
				TransactionStatusReversalPending, maxReversalPendingAttempts,
				TransactionStatusRefundPending, maxRefundPendingAttempts,
				TransactionStatusPending, stalePendingBefore, "",
			).
			Where("last_reconciliation_attempt_at IS NULL OR last_reconciliation_attempt_at < ?", leaseBefore).
			Order("created_at ASC").
			Limit(limit).
			Find(&txns).Error; err != nil {
			return err
		}

		if len(txns) == 0 {
			return nil
		}

		ids := make([]string, 0, len(txns))
		for i := range txns {
			ids = append(ids, txns[i].ID)
		}

		if err := tx.Model(&Transaction{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"reconciliation_attempts":        gorm.Expr("reconciliation_attempts + 1"),
				"last_reconciliation_attempt_at": now,
			}).Error; err != nil {
			return err
		}

		for i := range txns {
			txns[i].ReconciliationAttempts++
			txns[i].LastReconciliationAttemptAt = &now
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return txns, nil
}

func (r *Repository) MarkCashbackSettlementPending(ctx context.Context, txID string, balanceAfter int64) error {
	return r.db.WithContext(ctx).
		Model(&Transaction{}).
		Where("id = ?", txID).
		Updates(map[string]interface{}{
			"status":        TransactionStatusSuccessful,
			"balance_after": balanceAfter,
			"metadata": map[string]any{
				"cashback_settlement": "pending",
			},
		}).Error
}

func (r *Repository) ClearCashbackSettlementPending(ctx context.Context, txID string) error {
	return r.db.WithContext(ctx).Model(&Transaction{}).Where("id = ?", txID).
		Update("metadata", nil).Error
}

func (r *Repository) ReserveCashbackSpend(ctx context.Context, txID, mobileUserID string, requestedKobo int64, source string) (int64, error) {
	if requestedKobo <= 0 {
		return 0, nil
	}

	var reserved int64
	err := r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		var user models.User
		if err := txDB.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ?", mobileUserID).
			First(&user).Error; err != nil {
			return err
		}

		var existing models.Cashback
		if err := txDB.Where("transaction_id = ? AND entry_type = ?", txID, models.CashbackEntryDebit).
			First(&existing).Error; err == nil {
			reserved = existing.CashbackAmount
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var last models.Cashback
		before := int64(0)
		if err := txDB.Where("mobile_user_id = ?", mobileUserID).
			Order("created_at DESC").First(&last).Error; err == nil {
			before = last.CashbackAfter
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if before <= 0 {
			return txDB.Model(&Transaction{}).Where("id = ?", txID).Updates(map[string]interface{}{
				"used_cashback":   false,
				"cashback_amount": 0,
			}).Error
		}
		reserved = requestedKobo
		if reserved > before {
			reserved = before
		}
		after := before - reserved
		if err := txDB.Create(&models.Cashback{
			ID:             txID + "-cashback",
			MobileUserID:   mobileUserID,
			CashbackBefore: before,
			CashbackAfter:  after,
			CashbackAmount: reserved,
			Source:         source,
			EntryType:      models.CashbackEntryDebit,
			TransactionID:  &txID,
			CreatedAt:      time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		return txDB.Model(&Transaction{}).Where("id = ?", txID).Updates(map[string]interface{}{
			"used_cashback":   true,
			"cashback_amount": reserved,
		}).Error
	})
	return reserved, err
}

func (r *Repository) ReleaseCashbackSpend(ctx context.Context, txID, mobileUserID, source string) error {
	return r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		var user models.User
		if err := txDB.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").Where("id = ?", mobileUserID).First(&user).Error; err != nil {
			return err
		}

		var debit models.Cashback
		if err := txDB.Where("transaction_id = ? AND entry_type = ?", txID, models.CashbackEntryDebit).
			First(&debit).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var reversal models.Cashback
		if err := txDB.Where("transaction_id = ? AND entry_type = ?", txID, models.CashbackEntryReversal).
			First(&reversal).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var last models.Cashback
		if err := txDB.Where("mobile_user_id = ?", mobileUserID).
			Order("created_at DESC").First(&last).Error; err != nil {
			return err
		}
		after := last.CashbackAfter + debit.CashbackAmount
		return txDB.Create(&models.Cashback{
			ID:             txID + "-cashback-reversal",
			MobileUserID:   mobileUserID,
			CashbackBefore: last.CashbackAfter,
			CashbackAfter:  after,
			CashbackAmount: debit.CashbackAmount,
			Source:         source,
			EntryType:      models.CashbackEntryReversal,
			TransactionID:  &txID,
			CreatedAt:      time.Now().UTC(),
		}).Error
	})
}

func (r *Repository) StoreVASAsBeneficiary(ctx context.Context, beneficiary *VASBeneficiary) error {
	return r.db.WithContext(ctx).Create(beneficiary).Error
}

func (r *Repository) FetchVASBeneficiaries(ctx context.Context, mobileUserID, biller string) ([]VAS, error) {
	var beneficiaries []VAS
	err := r.db.WithContext(ctx).
		Where("mobile_user_id = ? AND billing_company = ?", mobileUserID, biller).
		Find(&beneficiaries).Error
	if err != nil {
		return nil, err
	}
	return beneficiaries, nil
}

func (r *Repository) GetLatestCashbackBalance(ctx context.Context, mobileUserID string) (int64, error) {
	var cashback models.Cashback
	err := r.db.WithContext(ctx).
		Where("mobile_user_id = ?", mobileUserID).
		Order("created_at DESC").
		First(&cashback).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return cashback.CashbackAfter, nil
}

func (r *Repository) CompleteCashbackSpend(ctx context.Context, txID, mobileUserID string, cashbackKobo int64, source string, status TransactionStatus, balanceAfter int64) (int64, error) {
	var after int64
	err := r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		var user models.User
		if err := txDB.WithContext(ctx).
			Select("id", "wallet_id").
			Where("id = ?", mobileUserID).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&user).Error; err != nil {
			return err
		}

		var reserved models.Cashback
		if err := txDB.Where("transaction_id = ? AND entry_type = ?", txID, models.CashbackEntryDebit).
			First(&reserved).Error; err == nil {
			after = reserved.CashbackAfter
			return txDB.Model(&Transaction{}).
				Where("id = ?", txID).
				Updates(map[string]interface{}{
					"used_cashback":   true,
					"cashback_amount": reserved.CashbackAmount,
					"status":          status,
					"balance_after":   balanceAfter,
				}).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var last models.Cashback
		var before int64
		switch err := txDB.WithContext(ctx).
			Where("mobile_user_id = ?", mobileUserID).
			Order("created_at DESC").
			First(&last).Error; {
		case err == nil:
			before = last.CashbackAfter
		case err == gorm.ErrRecordNotFound:
			before = 0
		default:
			return err
		}

		if before < cashbackKobo {
			return appErr.ErrInsufficientCashback
		}
		after = before - cashbackKobo

		cashbackRow := &models.Cashback{
			ID:             txID + "-cashback",
			MobileUserID:   mobileUserID,
			CashbackBefore: before,
			CashbackAfter:  after,
			CashbackAmount: cashbackKobo,
			Source:         source,
			EntryType:      models.CashbackEntryDebit,
			TransactionID:  &txID,
			CreatedAt:      time.Now().UTC(),
		}
		if err := txDB.WithContext(ctx).Create(cashbackRow).Error; err != nil {
			return err
		}

		return txDB.WithContext(ctx).
			Model(&Transaction{}).
			Where("id = ?", txID).
			Updates(map[string]interface{}{
				"used_cashback":   true,
				"cashback_amount": cashbackKobo,
				"status":          status,
				"balance_after":   balanceAfter,
			}).Error
	})
	if err != nil {
		if errors.Is(err, appErr.ErrInsufficientCashback) {
			return 0, appErr.ErrInsufficientCashback
		}
		return 0, err
	}
	return after, nil
}
