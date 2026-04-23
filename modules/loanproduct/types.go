package loanproduct

import "time"

type LoanFrequency string

const (
	LoanFrequencyWeekly  = "weekly"
	LoanFrequencyMonthly = "monthly"
)

type LoanDecison string

const (
	LoanDecisonEligible      LoanDecison = "eligible"
	LoanDecisionIneligible   LoanDecison = "ineligible"
	LoanDecisionManualReview LoanDecison = "manual_review"
)

type ApprovalLevel string

const (
	ApprovalLevelRelationOfficer ApprovalLevel = "relational_officer"
	ApprovalLevelBranchManager   ApprovalLevel = "branch_manager"
	ApprovalLevelCreditUnit      ApprovalLevel = "credit_unit"
)

type LoanType string

const (
	LoanTypeBusiness   LoanType = "BUSINESS-WK"
	LoanTypeSpecial    LoanType = "SPECIAL-WK"
	LoanTypeSME        LoanType = "SME-WK"
	LoanTypeSalary     LoanType = "SALARY-MTH"
	LoanTypeIndividual LoanType = "INDIVIDUAL-WK"
	LoanTypeGroup      LoanType = "GROUP-WK"
)

type LoanStatus string

const (
	LoanStatusEmbryo   LoanStatus = "embryo"
	LoanStatusPending  LoanStatus = "pending"
	LoanStatusApproved LoanStatus = "approved"
	LoanStatusDeclined LoanStatus = "declined"
	LoanStatusActive   LoanStatus = "active"
)

type CoreCustomerMatchStatus string

const (
	CoreCustomerNoMatch         CoreCustomerMatchStatus = "no_match"
	CoreCustomerSingleMatch     CoreCustomerMatchStatus = "single_match"
	CoreCustomerMultipleMatches CoreCustomerMatchStatus = "multiple_matches"
)

type CoreMatchedCustomer struct {
	CustomerID string `json:"customer_id"`
	FullName   string `json:"full_name"`
}

type CoreCustomerMatchData struct {
	MatchStatus CoreCustomerMatchStatus `json:"match_status"`
	Customer    *CoreMatchedCustomer    `json:"customer"`
}

type CoreCustomerLoanItem struct {
	LoanID             string  `json:"loan_id"`
	LoanNumber         string  `json:"loan_number"`
	PrincipalAmount    float64 `json:"principal_amount"`
	DisbursedAmount    float64 `json:"disbursed_amount"`
	OutstandingBalance float64 `json:"outstanding_balance"`
	Status             string  `json:"status"`
	NextDueDate        string  `json:"next_due_date"`
	NextDueAmount      float64 `json:"next_due_amount"`
}

type CoreLoanDetail struct {
	LoanID             string  `json:"loan_id"`
	LoanNumber         string  `json:"loan_number"`
	PrincipalAmount    float64 `json:"principal_amount"`
	DisbursedAmount    float64 `json:"disbursed_amount"`
	OutstandingBalance float64 `json:"outstanding_balance"`
	AccruedInterest    float64 `json:"accrued_interest"`
	Status             string  `json:"status"`
	NextDueDate        string  `json:"next_due_date"`
	NextDueAmount      float64 `json:"next_due_amount"`
}

type PartialLoanProduct struct {
	ID                    string        `json:"id"`
	Code                  string        `json:"code"`
	Name                  string        `json:"name"`
	Description           string        `json:"description"`
	MinLoanAmount         int64         `json:"min_loan_amount"`
	MaxLoanAmount         int64         `json:"max_loan_amount"`
	InterestRateBPS       int           `json:"interest_rate_bps"`
	RepaymentFrequency    LoanFrequency `json:"repayment_frequency"`
	GracePeriodDays       int           `json:"grace_period_days"`
	LoanTermValue         int           `json:"loan_term_value"`
	LatePenaltyBPS        int           `json:"late_penalty_bps"`
	AllowsConcurrentLoans *bool         `json:"allows_concurrent_loans"`
	IsActive              *bool         `json:"is_active"`
}

type LoanSummary struct {
	Amount            float64
	RatePercent       float64
	InterestAmount    float64
	TotalRepayment    float64
	PeriodicRepayment float64
}

type LoanRepayment struct {
	RepaymentID          string     `json:"repayment_id"`
	LoanID               string     `json:"loan_id"`
	ExpectedDateToBePaid string     `json:"expected_date_to_be_paid"`
	Amount               int64      `json:"amount"`
	Principal            int64      `json:"principal"`
	Profit               int64      `json:"profit"`
	Paid                 *bool      `json:"paid"`
	DatePaid             *time.Time `json:"date_paid"`
	DefaultedDate        *time.Time `json:"defaulted_date"`
	Status               string     `json:"status"`
}

type LoanSummaryRow struct {
	LoanProductType   string  `json:"loan_product_type"   gorm:"column:loan_product_type"`
	LoanAmount        float64 `json:"loan_amount"         gorm:"column:loan_amount"`
	TotalRepayment    float64 `json:"total_repayment"     gorm:"column:total_repayment"`
	PeriodicRepayment float64 `json:"periodic_repayment"  gorm:"column:periodic_repayment"`
	LoanDuration      string  `json:"loan_duration"       gorm:"column:loan_duration"`
	InterestRate      float64 `json:"interest_rate"       gorm:"column:interest_rate"`
}
