package loanproduct

import "time"

type LoanProduct struct {
	ID                    string        `gorm:"column:id;type:text;primaryKey" json:"id"`
	Code                  string        `gorm:"column:code;type:text;uniqueIndex;not null" json:"code"`
	Name                  string        `gorm:"column:name;type:text;not null"  json:"name"`
	Description           string        `gorm:"column:description;type:text;not null"  json:"description"`
	MinLoanAmount         int64         `gorm:"column:min_loan_amount;not null"  json:"min_loan_amount"`
	MaxLoanAmount         int64         `gorm:"column:max_loan_amount;not null"  json:"max_loan_amount"`
	InterestRateBPS       int           `gorm:"column:interest_rate_bps;not null"  json:"interest_rate_bps"`
	RepaymentFrequency    LoanFrequency `gorm:"column:repayment_frequency;type:text;not null"  json:"repayment_frequency"`
	GracePeriodDays       int           `gorm:"column:grace_period_days;not null;default:0" json:"grace_period_days"`
	LatePenaltyBPS        int           `gorm:"column:late_penalty_bps;not null;default:0" json:"late_penalty_bps"`
	AllowsConcurrentLoans bool          `gorm:"column:allows_concurrent_loans;not null;default:false" json:"allows_concurrent_loans"`
	IsActive              bool          `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt             time.Time     `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time     `gorm:"column:updated_at;type:timestamptz;not null;autoCreateTime" json:"updated_at"`
}

func (LoanProduct) TableName() string {
	return "loan_products"
}
