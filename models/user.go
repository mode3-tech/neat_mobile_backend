package models

import "time"

type User struct {
	ID              string    `gorm:"column:id;primaryKey;index"`
	Email           string    `gorm:"column:email;unique"`
	Phone           string    `gorm:"column:phone;unique"`
	PasswordHash    string    `gorm:"column:password"`
	PinHash         string    `gorm:"column:pin_hash"`
	IsEmailVerified bool      `gorm:"is_email_verified"`
	IsPhoneVerified bool      `gorm:"is_phone_verified"`
	IsBvnVerified   bool      `gorm:"is_bvn_verified"`
	IsNinVerified   bool      `gorm:"is_nin_verified"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (User) TableName() string {
	return "wallet_users"
}
