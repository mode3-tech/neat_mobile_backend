package vas

import (
	"context"
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

func (r *Repository) UpdateTransactionMetadata(ctx context.Context, txID string, metadata map[string]any) error {
	return r.db.WithContext(ctx).
		Model(&Transaction{}).
		Where("id = ?", txID).
		Update("metadata", metadata).Error
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
			return gorm.ErrRecordNotFound
		}
		after = before - cashbackKobo

		cashbackRow := &models.Cashback{
			ID:             txID + "-cashback",
			MobileUserID:   mobileUserID,
			CashbackBefore: before,
			CashbackAfter:  after,
			CashbackAmount: cashbackKobo,
			Source:         source,
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
		if err == gorm.ErrRecordNotFound {
			return 0, gorm.ErrRecordNotFound
		}
		return 0, err
	}
	return after, nil
}
