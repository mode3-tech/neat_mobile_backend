package models

import "time"

type User struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Email        string    `gorm:"column:email;unique"`
	Phone        string    `gorm:"column:phone;unique"`
	PasswordHash string    `gorm:"column:password"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (User) TableName() string {
	return "wallet_users"
}
