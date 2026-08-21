package models

import "time"

type Cashback struct {
	ID             string    `gorm:"column:id;type:text;primaryKey"`
	MobileUserID   string    `gorm:"column:mobile_user_id;not null"`
	CashbackBefore int64     `gorm:"column:cashback_before;type:bigint"`
	CashbackAfter  int64     `gorm:"column:cashback_after;type:bigint"`
	Source         string    `gorm:"column:cashback_source;type:text"` //e.g referral code, vas
	CreatedAt      time.Time `gorm:"column:created_at;type:timestamptz;autoCreateTime"`
}

func (Cashback) TableName() string {
	return "wallet_cashbacks"
}
