package referrals

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

func (r *Repository) FindReferralByCode(ctx context.Context, code string) (*ReferralCode, error) {
	var referral ReferralCode
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&referral).Error; err != nil {
		return nil, err
	}
	return &referral, nil
}

func (r *Repository) RedeemReferral(ctx context.Context, redeemedReferral *ReferralRedemption) error {
	return r.db.WithContext(ctx).Create(redeemedReferral).Error
}
