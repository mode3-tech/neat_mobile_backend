package models

import "time"

type UserDevice struct {
	ID          string    `gorm:"id;primaryKey"`
	UserID      string    `gorm:"user_id;type:text"`
	DeviceID    string    `gorm:"device_id;type:text"`
	PublicKey   string    `gorm:"public_key"`
	DeviceName  string    `gorm:"device_name;type:text"`
	DeviceModel string    `gorm:"device_model"`
	OS          string    `gorm:"os"`
	OSVersion   string    `gorm:"os_version"`
	AppVersion  string    `gorm:"app_version"`
	IsTrusted   bool      `gorm:"is_trusted"`
	IsActive    bool      `gorm:"is_active"`
	LastUsedAt  time.Time `gorm:"last_used_at"`
	CreatedAt   time.Time `gorm:"created_at"`
}

func (UserDevice) TableName() string {
	return "user_devices"
}
