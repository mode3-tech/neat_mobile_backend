package account

import "time"

type AccountSummaryResponse struct {
	Status bool           `json:"status"`
	Data   AccountSummary `json:"data"`
}

type AccountSummary struct {
	FullName         string       `json:"full_name"`
	Email            string       `json:"email,omitempty"`
	PhoneNumber      string       `json:"phone_number"`
	DOB              time.Time    `json:"dob"`
	Address          string       `json:"address"`
	BVN              string       `json:"bvn"`
	BankName         string       `json:"bank_name"`
	AccountNumber    string       `json:"account_number"`
	AvailableBalance int64        `json:"available_balance"`
	WalletID         string       `json:"wallet_id"`
	LoanBalance      float64      `json:"loan_balance"`
	ActiveLoans      []ActiveLoan `json:"active_loans"`
}

type ActiveLoan struct {
	LoanID           string  `json:"loan_id"`
	LoanNumber       string  `json:"loan_number"`
	LoanAmount       float64 `json:"loan_amount"`
	TotalRepayment   float64 `json:"total_repayment"`
	MonthlyRepayment float64 `json:"monthly_repayment"`
	NextDueDate      string  `json:"next_due_date"`
}

type AccountStatementRequest struct {
	Format   ReportFormat `json:"format" binding:"required"`
	DateFrom time.Time    `json:"date_from" binding:"required"`
	DateTo   time.Time    `json:"date_to" binding:"required"`
}

type AccountStatementResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	JobID   string `json:"job_id,omitempty"`
}

type StatementJobStatusResponse struct {
	Status      bool   `json:"status"`
	JobStatus   string `json:"job_status"`
	DownloadURL string `json:"download_url,omitempty"`
}

type UpdateProfileRequest struct {
	Email   *string `json:"email" binding:"omitempty,email"`
	Address *string `json:"address"`
}

type UpdateProfileResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
}
