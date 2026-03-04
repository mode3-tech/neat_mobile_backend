package models

import "time"

type DeviceChallenge struct {
	ID        string `gorm:"id;primaryKey"`
	UserID    string `gorm:"user_id;not null"`
	DeviceID  string `gorm:"device_id;not null"`
	Challenge string `gorm:"challenge;type:text;not null"`

	//replay protection
	UsedAt *time.Time `gorm:"type:timestamptz;not null;default:now()"`

	//expiry
	ExpiresAt time.Time `gorm:"type:timestamptz;not null;index" json:"expires_at"`

	UpdatedAt time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	CreatedAt time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	DeletedAt *time.Time `gorm:"type:timestamptz;not null"`
}

func (DeviceChallenge) TableName() string {
	return "device_challenges"
}

// Helpers
func (dc *DeviceChallenge) IsExpired(now time.Time) bool {
	return now.After(dc.ExpiresAt)
}

func (dc *DeviceChallenge) IsUsed() bool {
	return dc.UsedAt != nil
}
