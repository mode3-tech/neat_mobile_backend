package user

import (
	"context"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	DB *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) GetUserByUserID(ctx context.Context, mobileUserID string) (*UserWithAddress, error) {
	var result UserWithAddress
	err := r.DB.WithContext(ctx).
		Table("wallet_users").
		Select(`wallet_users.id,
		        wallet_users.first_name || ' ' || wallet_users.last_name as full_name,
		        wallet_users.phone, wallet_users.address,
		        wallet_bvn_records.full_home_address as bvn_home_address`).
		Joins("LEFT JOIN wallet_bvn_records ON wallet_bvn_records.user_id = wallet_users.id").
		Where("wallet_users.id = ?", mobileUserID).
		First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Repository) SetUserAsClosed(ctx context.Context, mobileUserID string, closedAt time.Time) error {
	err := r.DB.WithContext(ctx).
		Table("wallet_users").
		Where("id = ?", mobileUserID).
		Update("closed_at", closedAt).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) GetUserDetails(ctx context.Context, mobileUserID string) (*models.User, error) {
	var user models.User
	err := r.DB.WithContext(ctx).
		Table("wallet_users").
		Where("id = ?", mobileUserID).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
