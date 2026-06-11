package vas

type VASBeneficiary struct {
	ID             string `gorm:"column:id;type:text;primaryKey"`
	MobileUserID   string `gorm:"column:mobile_user_id;type:text;not null"`
	PhoneNumber    string `gorm:"column:phone_number;type:text"`
	Email          string `gorm:"column:email;type:text"`
	BillingCompany string `gorm:"column:billing_company;type:text;not null"`
	AccountNumber  string `gorm:"column:account_number;type:text;not null"`
	AccountType    string `gorm:"column:account_type;type:text;not null"`
	CreatedAt      string `gorm:"column:created_at;type:timestamptz;autoCreateTime;not null"`
	UpdatedAt      string `gorm:"column:updated_at;type:timestamptz;autoUpdateTime;"`
}

func (VASBeneficiary) TableName() string {
	return "wallet_vas_beneficiaries"
}
