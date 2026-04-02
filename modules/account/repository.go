package account

import (
	"context"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/device"
	"neat_mobile_app_backend/modules/wallet"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error) {
	var d device.UserDevice
	err := r.db.WithContext(ctx).Where("user_id = ? AND device_id = ?", mobileUserID, deviceID).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) GetUser(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Select("id", "first_name", "last_name").
		Where("id = ?", userID).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetCustomerWallet(ctx context.Context, mobileUserID string) (*wallet.CustomerWallet, error) {
	var w wallet.CustomerWallet
	err := r.db.WithContext(ctx).
		Select("account_number", "available_balance", "booked_balance", "internal_wallet_id").
		Where("mobile_user_id = ?", mobileUserID).
		First(&w).Error
	if err != nil {
		return nil, err
	}
	return &w, nil
}
