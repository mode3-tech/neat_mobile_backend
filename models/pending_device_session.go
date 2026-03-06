package models

import (
	"time"
)

type PendingDeviceSession struct {
	ID               string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID           string     `gorm:"type:uuid;not null;index" json:"user_id"`
	DeviceID         string     `gorm:"type:varchar(128);not null;index" json:"device_id"`
	SessionTokenHash string     `gorm:"type:char(64);not null;uniqueIndex" json:"-"`
	OTPRef           string     `gorm:"type:varchar(128)" json:"otp_ref,omitempty"`
	VerifiedAt       *time.Time `gorm:"type:timestamptz" json:"verified_at,omitempty"`
	UsedAt           *time.Time `gorm:"type:timestamptz;default:null" json:"used_at"`
	ExpiresAt        time.Time  `gorm:"type:timestamptz;not null;index" json:"expires_at"`

	// Helpful for security/auditing (optional)
	IP        string `gorm:"type:varchar(64)" json:"ip,omitempty"`
	UserAgent string `gorm:"type:text" json:"user_agent,omitempty"`

	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:null" json:"updated_at"`
}

func (PendingDeviceSession) TableName() string { return "pending_device_sessions" }

// Helpers
func (p *PendingDeviceSession) IsExpired(now time.Time) bool {
	return now.After(p.ExpiresAt)
}

func (p *PendingDeviceSession) IsVerified() bool {
	return p.VerifiedAt != nil
}

func (p *PendingDeviceSession) IsUsed() bool {
	return p.UsedAt != nil
}
