package card

import (
	"context"
	"neat_mobile_app_backend/internal/modules/wallet"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindWalletWithMobileUserID(ctx context.Context, mobileUserID string) (*wallet.CustomerWallet, error) {
	var wallet wallet.CustomerWallet
	err := r.db.WithContext(ctx).
		Select("account_number, account_name, phone_number, internal_wallet_id, available_balance, address, bvn").
		Where("mobileUserID = ?", mobileUserID).
		First(&wallet).Error

	if err != nil {
		return nil, err
	}

	return &wallet, nil
}
