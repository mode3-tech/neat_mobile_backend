package models

import "time"

type PushToken struct {
	ID            string    `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	UserID        string    `gorm:"column:user_id;type:text;not null;uniqueIndex:uq_wallet_push_tokens_user_device"`
	DeviceID      string    `gorm:"column:device_id;type:text;not null;uniqueIndex:uq_wallet_push_tokens_user_device"`
	ExpoPushToken string    `gorm:"column:expo_push_token;type:text;not null;uniqueIndex:uq_wallet_push_tokens_token"`
	Platform      string    `gorm:"column:platform;type:text;not null;check:platform IN ('ios', 'android')"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (PushToken) TableName() string {
	return "wallet_push_tokens"
}
