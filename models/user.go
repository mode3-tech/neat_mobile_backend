package models

import "time"

type User struct {
	ID           string    `gorm:"column:id"`
	Email        string    `gorm:"column:email"`
	PasswordHash string    `gorm:"column:password"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}
