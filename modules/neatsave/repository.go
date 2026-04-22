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

func (r *Repository) CreateGoalWithRules(ctx context.Context, savingsGoal *SavingsGoal, rules *AutoSaveRule) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(savingsGoal).Error; err != nil {
			return err
		}
		if rules != nil {
			if err := tx.Create(rules).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
