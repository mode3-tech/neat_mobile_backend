package transaction

type TransactionType string

const (
	TransactionTypeDebit  TransactionType = "debit"
	TransactionTypeCredit TransactionType = "credit"
)

type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "pending"
	TransactionStatusSuccessful TransactionStatus = "successful"
	TransactionStatusFailed     TransactionStatus = "failed"
	TransactionStatusReversed   TransactionStatus = "reversed"
)

type TransactionSource string

const (
	TransactionSourceDebit            TransactionSource = "debit"
	TransactionSourceCredit           TransactionSource = "credit"
	TransactionSourceLoanDisbursement TransactionSource = "loan_disbursement"
)
