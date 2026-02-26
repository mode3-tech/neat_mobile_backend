package otp

import "time"

type OTPModel struct {
	ID           string     `gorm:"column:id;type:text;primaryKey"`
	UserID       string     `gorm:"column:user_id;type:text;index"`
	Purpose      Purpose    `gorm:"column:purpose;type:text;index"`
	Channel      Channel    `gorm:"column:channel;type:text"`
	Destination  string     `gorm:"column:destination;type:text;not null;index"`
	OTPHash      string     `gorm:"column:otp_hash;type:text;not null"`
	ExpiresAt    time.Time  `gorm:"column:expires_at;index"`
	RequestID    string     `gorm:"column:request_id"`
	ConsumedAt   *time.Time `gorm:"column:consumed_at;index"`
	AttemptCount int        `gorm:"column:attempt_count"`
	MaxAttempts  int        `gorm:"column:max_attempts"`
	ResendCount  int        `gorm:"column:resend_count"`
	MaxResends   int        `gorm:"column:max_resends"`
	NextSendAt   *time.Time `gorm:"column:next_send_at"`
	IssuedAt     time.Time  `gorm:"column:issued_at; not null;autoCreateTime"`
}
