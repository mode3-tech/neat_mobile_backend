package referrals

import "time"

const (
	CashbackStatusPending  = "pending"
	CashbackStatusCredited = "credited"
)

type ReferralCode struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Code         string    `gorm:"column:code;type:text;not null;unique"`
	MobileUserID string    `gorm:"column:mobile_user_id;type:text;not null;unique"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
}

func (ReferralCode) TableName() string {
	return "wallet_referral_codes"
}

type ReferralRedemption struct {
	ID             string    `gorm:"column:id;primaryKey"`
	ReferrerUserID string    `gorm:"column:referrer_user_id;type:text;not null"`
	ReferredUserID string    `gorm:"column:referred_user_id;type:text;not null;unique"`
	CashbackStatus string    `gorm:"column:cashback_status;type:text;not null;default:'pending'"`
	CreatedAt      time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
}

func (ReferralRedemption) TableName() string {
	return "wallet_referral_redemptions"
}