package models

import "time"

type RefreshToken struct {
	JTI           string     `gorm:"column:jti;type:text;primaryKey"`
	SessionID     string     `gorm:"column:sid;type:text;index;not null"`
	UserID        string     `gorm:"column:user_id;type:text;index;not null"`
	TokenHash     string     `gorm:"column:token_hash;type:text;uniqueIndex;not null"`
	IssuedAt      time.Time  `gorm:"column:issued_at;not null"`
	ExpiresAt     time.Time  `gorm:"column:expires_at;index;not null"`
	LastUsedAt    *time.Time `gorm:"column:last_used_at"`
	RevokedAt     *time.Time `gorm:"column:revoked_at;index"`
	ReplacedByJTI *string    `gorm:"column:replaced_by_jti;type:text"`

	Session AuthSession `gorm:"foreignKey:SessionID;references:SID;constraint:OnDelete:CASCADE"`
}

func (RefreshToken) TableName() string {
	return "wallet_refresh_tokens"
}
