package device

import "time"

type UserDevice struct {
	ID          string    `gorm:"column:id;primaryKey"`
	UserID      string    `gorm:"column:user_id;type:text;not null;uniqueIndex:uq_user_devices_user_device"`
	DeviceID    string    `gorm:"column:device_id;type:text;not null;uniqueIndex:uq_user_devices_user_device"`
	PublicKey   string    `gorm:"column:public_key"`
	DeviceName  string    `gorm:"column:device_name;type:text"`
	DeviceModel string    `gorm:"column:device_model"`
	OS          string    `gorm:"column:os"`
	OSVersion   string    `gorm:"column:os_version"`
	IP          string    `gorm:"column:ip"`
	AppVersion  string    `gorm:"column:app_version"`
	IsTrusted   bool      `gorm:"column:is_trusted"`
	IsActive    bool      `gorm:"column:is_active"`
	LastUsedAt  time.Time `gorm:"column:last_used_at"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (UserDevice) TableName() string {
	return "user_devices"
}

type DeviceChallenge struct {
	ID            string `gorm:"column:id;primaryKey"`
	UserID        string `gorm:"column:user_id;not null"`
	DeviceID      string `gorm:"column:device_id;not null"`
	ChallengeHash string `gorm:"column:challenge_hash;index;type:text;not null"`

	//replay protection
	UsedAt *time.Time `gorm:"column:used_at;type:timestamptz;default:null"`

	//expiry
	ExpiresAt time.Time `gorm:"column:expires_at;type:timestamptz;not null;index" json:"expires_at"`

	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now()" json:"updated_at"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
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
