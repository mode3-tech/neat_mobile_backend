package transaction

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

func (r *Repository) FetchRecentTransactions(ctx context.Context, userID, walletID string) ([]Transaction, error) {
	var transactions []Transaction
	err := r.db.WithContext(ctx).
		Where("mobile_user_id = ? AND wallet_id = ?", userID, walletID).
		Order("created_at DESC").
		Limit(2).
		Find(&transactions).Error
	return transactions, err
}
