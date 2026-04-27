package autorepayment

type AutoRepaymentAttemptStatus string

const (
	AutoRepaymentAttemptStatusPending AutoRepaymentAttemptStatus = "pending"
	AutoRepaymentAttemptStatusSuccess AutoRepaymentAttemptStatus = "success"
	AutoRepaymentAttemptStatusFailed  AutoRepaymentAttemptStatus = "failed"
	AutoRepaymentAttemptStatusSkipped AutoRepaymentAttemptStatus = "skipped"
)
