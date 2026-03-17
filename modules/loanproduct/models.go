package loanproduct

import (
	"time"
)

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
	LoanTermValue         int           `gorm:"column:loan_term_value;not null" json:"loan_term_value"`
	LatePenaltyBPS        int           `gorm:"column:late_penalty_bps;not null;default:0" json:"late_penalty_bps"`
	AllowsConcurrentLoans bool          `gorm:"column:allows_concurrent_loans;not null;default:false" json:"allows_concurrent_loans"`
	IsActive              bool          `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt             time.Time     `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime" json:"created_at"`
	UpdatedAt             *time.Time    `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime" json:"updated_at"`
}

func (LoanProduct) TableName() string {
	return "wallet_loan_products"
}

type LoanProductRule struct {
	ID                          string `gorm:"column:id;type:text;primaryKey"`
	ProductID                   string `gorm:"column:product_id;type:text;foreignKey;not null"`
	MinSavingsBalance           int64  `gorm:"column:min_savings_balance;not null"`
	MinAccountAgeDays           int    `gorm:"column:min_account_age_days;not null;default:0"`
	MaxActiveLoans              int    `gorm:"column:max_account_loans;not null;default:0"`
	RequireKYC                  *bool  `gorm:"column:require_kyc"`
	RequireBVN                  *bool  `gorm:"required_bvn"`
	RequireNIN                  *bool  `gorm:"require_bvn"`
	RequirePhoneVerified        *bool  `gorm:"require_phone_verified"`
	RequireNoOutstandingDefault *bool  `gorm:"require_no_outstanding_default"`
	HighValueThreshold          int    `gorm:"high_value_threshold;not null"`
	BranchManagerApprovalLimit  int64  `gorm:"branch_manager_approval_limit;not null"`
}

func (LoanProductRule) TableName() string {
	return "wallet_loan_product_rules"
}

type LoanProductEvaluation struct {
	ID                    string        `gorm:"column:id;type:text;primaryKey"`
	ProductID             string        `gorm:"column:product_id;type:text;foreignKey;not null;index"`
	CustomerID            string        `gorm:"column:customer_id;type:text;foreignKey;not null;index"`
	RequestedAmount       int64         `gorm:"column:requested_amount;not null"`
	LoanTermValue         int           `gorm:"column:loan_term_value"`
	LoanTermUnit          LoanFrequency `gorm:"column:loan_term_unit;not null"`
	Decision              LoanDecison   `gorm:"column:decision"`
	RequiredApprovalLevel ApprovalLevel `gorm:"column:required_approval_level"`
	FailedCodes           string        `gorm:"column:failed_codes;type:jsonb;not null;default:'[]'" json:"failed_codes"`
	ResultJSON            string        `gorm:"column:result_json;type:jsonb;not null" json:"result_json"`
	EvaluatedBy           string        `gorm:"column:evaluted_by;not null"`
	CreatedAt             time.Time     `gorm:"column:created_at;type:timestamptz;autoCreateTime;not null"`
}

func (LoanProductEvaluation) TableName() string {
	return "wallet_loan_product_evaluations"
}

type LoanApplication struct {
	ID              string        `gorm:"column:id;type:text;primaryKey"`
	MobileUserID    string        `gorm:"column:mobile_user_id;type:text;not null;index"`
	CoreCustomerID  *string       `gorm:"column:core_customer_id"`
	PhoneNumber     string        `gorm:"column:phone_number;not null"`
	ApplicationRef  string        `gorm:"column:application_ref;not null;uniqueIndex"`
	CoreLoanID      *string       `gorm:"column:core_loan_id"`
	LoanProductType LoanType      `gorm:"column:loan_product_type;not null"`
	BusinessAddress string        `gorm:"column:business_address;not null"`
	BusinessValue   int64         `gorm:"column:business_value;not null;default:0"`
	BusinessType    string        `gorm:"column:business_type;not null"`
	RequestedAmount int64         `gorm:"column:requested_amount;not null;default:0"`
	LoanStatus      LoanStatus    `gorm:"column:loan_status;not null;default:pending"`
	Tenure          LoanFrequency `gorm:"column:tenure;not null"`
	TenureValue     int           `gorm:"column:tenure_value"`
	CreatedAt       time.Time     `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt       *time.Time    `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (LoanApplication) TableName() string {
	return "wallet_loan_applications"
}

type LoanApplicationStatusEvent struct {
	ID             string     `gorm:"column:id;type:text;primaryKey"`
	EventID        string     `gorm:"column:event_id;type:text;not null;uniqueIndex"`
	ApplicationRef string     `gorm:"column:application_ref;type:text;not null;index"`
	Status         LoanStatus `gorm:"column:status;type:text;not null"`
	CoreLoanID     *string    `gorm:"column:core_loan_id;type:text"`
	RawPayload     string     `gorm:"column:raw_payload;type:jsonb;not null"`
	ProcessedAt    time.Time  `gorm:"column:processed_at;type:timestamptz;not null"`
}

func (LoanApplicationStatusEvent) TableName() string {
	return "wallet_loan_application_status_events"
}
