package autorepayment

import "time"

type AutoRepaymentAttempt struct {
	ID              string                     `gorm:"column:id;primaryKey"`
	LoanRepaymentID int64                      `gorm:"column:loan_repayment_id;not null;index"`
	MobileUserID    string                     `gorm:"column:mobile_user_id;type:text;not null"`
	Amount          int64                      `gorm:"column:amount;not null"`
	Status          AutoRepaymentAttemptStatus `gorm:"column:status;type:text;not null"`
	FailureReason   string                     `gorm:"column:failure_reason;type:text"`
	ProviderRef     string                     `gorm:"column:provider_ref;type:text"`
	AttemptedAt     time.Time                  `gorm:"column:attempted_at;type:timestamptz;not null"`
}

func (AutoRepaymentAttempt) TableName() string {
	return "wallet_auto_repayment_attempts"
}
