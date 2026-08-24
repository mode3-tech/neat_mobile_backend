package referrals

import (
	"context"
	"neat_mobile_app_backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *Repository) GetLatestCashback(ctx context.Context, mobileUserID string) (*models.Cashback, error) {
	var cashback models.Cashback
	if err := r.db.WithContext(ctx).
		Where("mobile_user_id = ?", mobileUserID).
		Order("created_at DESC").
		First(&cashback).Error; err != nil {
		return nil, err
	}
	return &cashback, nil
}

func (r *Repository) CreateCashback(ctx context.Context, cashback *models.Cashback) error {
	return r.db.WithContext(ctx).Create(cashback).Error
}

func (r *Repository) GetUserWalletID(ctx context.Context, mobileUserID string) (string, error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Select("id", "wallet_id").
		Where("id = ?", mobileUserID).
		First(&user).Error; err != nil {
		return "", err
	}
	return user.WalletID, nil
}


func (r *Repository) GetUserWalletIDForUpdate(ctx context.Context, mobileUserID string) (string, error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Select("id", "wallet_id").
		Where("id = ?", mobileUserID).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&user).Error; err != nil {
		return "", err
	}
	return user.WalletID, nil
}

// WithTx runs fn with a repository bound to a database transaction. When the
// underlying db is already inside a transaction (e.g. called from registerv2's
// registration transaction), gorm uses savepoints so the work joins the outer
// transaction.
func (r *Repository) WithTx(ctx context.Context, fn func(txRepo *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		return fn(&Repository{db: txDB})
	})
}

func (r *Repository) FetchRedeemReferrals(ctx context.Context, page, pageSize int) ([]RedeemedReferral, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var redeemedReferrals []RedeemedReferral
	offset := (page - 1) * pageSize

	err := r.db.WithContext(ctx).Raw(`
		SELECT
			rr.id,
			CONCAT_WS(' ', referrer.first_name, referrer.last_name) AS referrer_name,
			CONCAT_WS(' ', referred.first_name, referred.last_name) AS referred_name,
			rr.created_at AS redeemed_at
		FROM wallet_referral_redemptions AS rr
		JOIN wallet_users AS referrer ON referrer.id = rr.referrer_user_id
		JOIN wallet_users AS referred ON referred.id = rr.referred_user_id
		ORDER BY rr.created_at DESC, rr.id DESC
		LIMIT ? OFFSET ?
	`, pageSize, offset).Scan(&redeemedReferrals).Error
	if err != nil {
		return nil, err
	}

	return redeemedReferrals, nil
}
