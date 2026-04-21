package neatsave

import (
	"context"
	"neat_mobile_app_backend/modules/device"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error) {
	var userDevice device.UserDevice

	if err := r.db.WithContext(ctx).
		Model(&device.UserDevice{}).
		Select("*").
		Where("user_id = ? AND device_id = ?", mobileUserID, deviceID).
		First(&userDevice).Error; err != nil {
		return nil, err
	}

	return &userDevice, nil
}

func (r *Repository) CreateGoal(ctx context.Context, savingsGoal *SavingsGoal) error {
	return r.db.WithContext(ctx).Create(&savingsGoal).Error
}
