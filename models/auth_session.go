package models

import "time"

type AuthSession struct {
	SID        string     `gorm:"column:sid;type:text;primaryKey"`
	UserID     string     `gorm:"column:user_id;type:text;index;not null"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	LastSeenAt *time.Time `gorm:"column:last_seen_at"`
	RevokedAt  *time.Time `gorm:"column:revoked_at;index"`
	DeviceID   *string    `gorm:"column:device_id;type:text"`
	UserAgent  *string    `gorm:"column:user_agent;type:text"`
	IP         *string    `gorm:"column:ip;type:text"`
}

func (AuthSession) TableName() string { return "wallet_auth_sessions" }
