package models

import "time"

type User struct {
	ID                  string    `gorm:"column:id;primaryKey;index"`
	Email               string    `gorm:"column:email;unique;not null"`
	Phone               string    `gorm:"column:phone;unique;index;not null"`
	PasswordHash        string    `gorm:"column:password;not null"`
	PinHash             string    `gorm:"column:pin_hash;not null"`
	IsEmailVerified     bool      `gorm:"is_email_verified"`
	IsPhoneVerified     bool      `gorm:"is_phone_verified;not null"`
	IsBvnVerified       bool      `gorm:"is_bvn_verified;not null"`
	IsNinVerified       bool      `gorm:"is_nin_verified;not null"`
	IsBiometricsEnabled *bool     `gorm:"is_biometrics_enabled"`
	CreatedAt           time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (User) TableName() string {
	return "wallet_users"
}
