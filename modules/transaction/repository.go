package transaction

import (
	"context"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FetchUserWithUserID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Model(models.User{}).Where("id = ?", userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, err
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

func (r *Repository) FetchTransactionPaged(ctx context.Context, userID, walletID string, cursor time.Time, limit int) ([]Transaction, error) {
	var txs []Transaction
	q := r.db.WithContext(ctx).
		Where("mobile_user_id = ? AND wallet_id = ?", userID, walletID)

	if !cursor.IsZero() {
		q = q.Where("created_at < ?", cursor)
	}

	err := q.Order("created_at DESC").Limit(limit + 1). // +1 to detect has_more
								Find(&txs).Error
	return txs, err
}
