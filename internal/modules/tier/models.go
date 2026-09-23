package tier

import "time"

type WalletAccountTier struct {
	ID                   string     `gorm:"type:text;primaryKey"`
	Tier                 int        `gorm:"type:int;unique"`
	DailyLimit           int64      `gorm:"type:bigint;not null"`
	MaxCumulativeBalance int64      `gorm:"type:bigint;not null"`
	Requirements         StringList `gorm:"type:jsonb"`
	CreatedAt            time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt            *time.Time `gorm:"type:timestamptz;"`
}

func (t *WalletAccountTier) TableName() string {
	return "wallet_account_tiers"
}

type TierSubmission struct {
	ID              string               `gorm:"type:text;primaryKey"`
	UserID          string               `gorm:"type:text;not null;index:idx_wallet_tier_submissions_user_requirement"`
	Requirement     TierRequirement      `gorm:"type:text;not null;index:idx_wallet_tier_submissions_user_requirement;check:requirement IN ('passport_photograph','utility_bill')"`
	DocumentURL     string               `gorm:"type:text;not null"`
	Status          TierSubmissionStatus `gorm:"type:text;not null;default:pending;check:status IN ('pending','approved','rejected')"`
	ReviewedBy      *string              `gorm:"type:text"`
	ReviewedAt      *time.Time           `gorm:"type:timestamptz"`
	RejectionReason *string              `gorm:"type:text"`
	CreatedAt       time.Time            `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt       *time.Time           `gorm:"type:timestamptz"`
}

func (t *TierSubmission) TableName() string {
	return "wallet_tier_submissions"
}
