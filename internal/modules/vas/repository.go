package vas

import (
	"context"

	"gorm.io/gorm"
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
