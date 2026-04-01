package account

type AccountSummaryResponse struct {
	Status bool           `json:"status"`
	Data   AccountSummary `json:"data"`
}

type AccountSummary struct {
	FullName         string       `json:"full_name"`
	AccountNumber    string       `json:"account_number"`
	AvailableBalance int64        `json:"available_balance"`
	LoanBalance      float64      `json:"loan_balance"`
	ActiveLoans      []ActiveLoan `json:"active_loans"`
}

type ActiveLoan struct {
	LoanID             string  `json:"loan_id"`
	LoanNumber         string  `json:"loan_number"`
	LoanAmount         float64 `json:"loan_amount"`
	TotalRepayment     float64 `json:"total_repayment"`
	MonthlyRepayment   float64 `json:"monthly_repayment"`
	NextDueDate        string  `json:"next_due_date"`
}
