package autorepayment

type AutoRepaymentAttemptStatus string

const (
	AutoRepaymentAttemptStatusPending AutoRepaymentAttemptStatus = "pending"
	AutoRepaymentAttemptStatusSuccess AutoRepaymentAttemptStatus = "success"
	AutoRepaymentAttemptStatusFailed  AutoRepaymentAttemptStatus = "failed"
	AutoRepaymentAttemptStatusSkipped AutoRepaymentAttemptStatus = "skipped"
)

type DueRepaymentRow struct {
	RepaymentID    int64  `json:"repayment_id"`
	LoanID         int64  `json:"loan_id"`
	Amount         int64  `json:"amount"`
	MobileUserID   string `json:"mobile_user_id"`
	CoreCustomerID int64  `json:"core_customer_id"`
}
