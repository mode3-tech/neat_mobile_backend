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
	TransactionSourceAutoSave         TransactionSource = "auto_save"
	TransactionSourceLoanRepayment    TransactionSource = "loan_repayment"
	TransactionSourceAutoRepayment    TransactionSource = "auto_repayment"
	TransactionSourceCard             TransactionSource = "card"
)

type TransactionCategory string

const (
	TransactionCategoryTransferFrom  TransactionCategory = "transfer_from"
	TransactionCategoryTransferTo    TransactionCategory = "transfer_to"
	TransactionCategoryAirtime       TransactionCategory = "airtime"
	TransactionCategoryMobileData    TransactionCategory = "mobile_data"
	TransactionCategoryReversal      TransactionCategory = "reversal"
	TransactionCategoryTV            TransactionCategory = "tv"
	TransactionCategoryElectricity   TransactionCategory = "electricity"
	TransactionCategoryCardPayment   TransactionCategory = "card_payment"
	TransactionCategoryLoanRepayment TransactionCategory = "loan_repayment"
)

var TransactionCategories = map[TransactionCategory]string{
	TransactionCategoryTransferFrom:  "Transfer From",
	TransactionCategoryTransferTo:    "Transfer To",
	TransactionCategoryAirtime:       "Airtime",
	TransactionCategoryMobileData:    "Mobile Data",
	TransactionCategoryReversal:      "Reversal",
	TransactionCategoryTV:            "TV",
	TransactionCategoryElectricity:   "Electricity",
	TransactionCategoryCardPayment:   "Card Payment",
	TransactionCategoryLoanRepayment: "Loan Repayment",
}
