package models

import "time"

type CustomerStatus string

const (
	CustomerStatusDraft    CustomerStatus = "draft"
	CustomerStatusEmbryo   CustomerStatus = "embryo"
	CustomerStatusPending  CustomerStatus = "pending"
	CustomerStatusApproved CustomerStatus = "approved"
)

type User struct {
	ID                           string          `gorm:"column:id;primaryKey;index"`
	WalletID                     string          `gorm:"column:wallet_id;uniqueIndex;not null"`
	FirstName                    string          `gorm:"column:first_name;not null"`
	LastName                     string          `gorm:"column:last_name;not null"`
	Address                      *string         `gorm:"column:address"`
	MiddleName                   *string         `gorm:"column:middle_name"`
	Email                        string          `gorm:"column:email;unique;not null"`
	Phone                        string          `gorm:"column:phone;unique;index;not null"`
	PasswordHash                 string          `gorm:"column:password;not null"`
	PinHash                      string          `gorm:"column:pin_hash;not null"`
	FailedTransactionPinAttempts int             `gorm:"column:failed_transaction_pin_attempts;not null;default:0"`
	TransactionPinLockedUntil    *time.Time      `gorm:"column:transaction_pin_locked_until"`
	DOB                          time.Time       `gorm:"column:dob;not null"`
	BVN                          string          `gorm:"column:bvn;not null"`
	NIN                          string          `gorm:"column:nin;not null"`
	CustomerStatus               *CustomerStatus `gorm:"column:customer_status;default:embryo"`
	Username                     *string         `gorm:"column:username"`
	CoreCustomerID               *string         `gorm:"column:core_customer_id"`
	IsEmailVerified              bool            `gorm:"is_email_verified"`
	IsPhoneVerified              bool            `gorm:"is_phone_verified;not null"`
	IsBvnVerified                bool            `gorm:"is_bvn_verified;not null"`
	IsNinVerified                bool            `gorm:"is_nin_verified;not null"`
	IsBiometricsEnabled          *bool           `gorm:"is_biometrics_enabled"`
	CreatedAt                    time.Time       `gorm:"column:created_at;autoCreateTime"`
}

func (User) TableName() string {
	return "wallet_users"
}
