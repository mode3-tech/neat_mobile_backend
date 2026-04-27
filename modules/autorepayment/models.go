package autorepayment

import "time"

type AutoRepaymentAttempt struct {
	ID              string                     `gorm:"column:id;primaryKey;index"`
	LoanRepaymentID int64                      `gorm:"column:loan_repayment_id;uniqueIndex;not null"`
	MobileUserID    string                     `gorm:"mobile_user_id;type:text;not null"`
	Amount          int64                      `gorm:"column:amount; not null"`
	Status          AutoRepaymentAttemptStatus `gorm:"status;type:text;not null"`
	FailureReason   string                     `gorm:"column:failure_reason;type:text"`
	ProviderRef     string                     `gorm:"column:provider_ref"`
	AttemptedAt     time.Time                  `gorm:"column:attempted_at;type:timestamptz;not null"`
}
