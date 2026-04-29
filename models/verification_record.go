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
	ID                      string     `gorm:"column:id;type:text;primaryKey"`
	Type                    string     `gorm:"column:type;type:text;index;not null"`
	Provider                string     `gorm:"column:provider;type:text;index"`
	Status                  string     `gorm:"column:status;type:text;index;not null"`
	SubjectHash             string     `gorm:"column:subject_hash;type:text;index;not null"`
	SubjectMasked           *string    `gorm:"column:subject_masked;type:text"`
	ProviderVerificationID  *string    `gorm:"column:provider_verification_id;type:text;index"`
	ReferenceID             *string    `gorm:"column:reference_id;type:text;index"`
	VerifiedName            *string    `gorm:"column:verified_name;type:text"`
	VerifiedPhone           *string    `gorm:"column:verified_phone;type:text"`
	VerifiedEmail           *string    `gorm:"column:verified_email;type:text"`
	VerifiedID              *string    `gorm:"column:verified_id"`
	VerifiedDOB             *string    `gorm:"column:verified_dob;type:text"`
	VerifiedGender          *string    `gorm:"column:verified_gender;type:text"`
	VerifiedNationality     *string    `gorm:"column:verified_nationality;type:text"`
	VerifiedStateOfOrigin   *string    `gorm:"column:verified_state_of_origin;type:text"`
	VerifiedPlaceOfBirth    *string    `gorm:"column:verified_place_of_birth;type:text"`
	VerifiedOccupation      *string    `gorm:"column:verified_occupation;type:text"`
	VerifiedMaritalStatus   *string    `gorm:"colimn:verified_marital_status;type:text"`
	VerifiedEducation       *string    `gorm:"column:verified_education;type:text"`
	VerifiedReligion        *string    `gorm:"column:verified_religion;type:text"`
	PassportOnBVN           *string    `gorm:"column:passport_on_bvn"`
	Passport                *string    `gorm:"column:passport"`
	VerifiedFullHomeAddress *string    `gorm:"column:verified_full_home_address;type:text"`
	TypeOfHouse             *string    `gorm:"column:type_of_house;type:text"`
	City                    *string    `gorm:"column:city;type:text"`
	Landmark                *string    `gorm:"column:landmark;type:text"`
	LivingSince             *time.Time `gorm:"living_since"`
	AlternativeMobilePhone  *string    `gorm:"alternative_mobile_phone"`
	BankName                string     `gorm:"column:bank_name;type:text;not null"`
	AccountNumber           *string    `gorm:"column:account_number;type:text;index"`
	FailureReason           *string    `gorm:"column:failure_reason;type:text"`
	CreatedAt               time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt               time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
	VerifiedAt              *time.Time `gorm:"column:verified_at;index"`
	ExpiresAt               *time.Time `gorm:"column:expires_at;index"`
	UsedAt                  *time.Time `gorm:"column:used_at;index"`
}

func (VerificationRecord) TableName() string {
	return "wallet_verification_records"
}
