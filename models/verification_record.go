package models

import "time"

const (
	VerificationTypeBVN   = "bvn"
	VerificationTypeNIN   = "nin"
	VerificationTypePhone = "phone"
	VerificationTypeEmail = "email"
)

const (
	VerificationStatusPending  = "pending"
	VerificationStatusVerified = "verified"
	VerificationStatusFailed   = "failed"
	VerificationStatusUsed     = "used"
	VerificationStatusExpired  = "expired"
)

type VerificationRecord struct {
	ID                     string     `gorm:"column:id;type:text;primaryKey"`
	Type                   string     `gorm:"column:type;type:text;index;not null"`
	Provider               string     `gorm:"column:provider;type:text;index"`
	Status                 string     `gorm:"column:status;type:text;index;not null"`
	SubjectHash            string     `gorm:"column:subject_hash;type:text;index;not null"`
	SubjectMasked          *string    `gorm:"column:subject_masked;type:text"`
	ProviderVerificationID *string    `gorm:"column:provider_verification_id;type:text;index"`
	ReferenceID            *string    `gorm:"column:reference_id;type:text;index"`
	VerifiedName           *string    `gorm:"column:verified_name;type:text"`
	VerifiedPhone          *string    `gorm:"column:verified_phone;type:text"`
	VerifiedEmail          *string    `gorm:"column:verified_email;type:text"`
	VerifiedDOB            *string    `gorm:"column:verified_dob;type:text"`
	FailureReason          *string    `gorm:"column:failure_reason;type:text"`
	CreatedAt              time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt              time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
	VerifiedAt             *time.Time `gorm:"column:verified_at;index"`
	ExpiresAt              *time.Time `gorm:"column:expires_at;index"`
	UsedAt                 *time.Time `gorm:"column:used_at;index"`
}

func (VerificationRecord) TableName() string {
	return "wallet_verification_records"
}
