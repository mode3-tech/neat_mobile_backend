package account

import "time"

type AccountSummaryResponse struct {
	Data AccountSummary `json:"data"`
}

type AccountSummary struct {
	FullName               string       `json:"full_name"`
	Email                  string       `json:"email,omitempty"`
	PhoneNumber            string       `json:"phone_number"`
	ProfilePicture         string       `json:"profile_picture"`
	DOB                    time.Time    `json:"dob"`
	Address                string       `json:"address"`
	BVN                    string       `json:"bvn"`
	BankName               string       `json:"bank_name"`
	AccountNumber          string       `json:"account_number"`
	AvailableBalance       int64        `json:"available_balance"`
	LoanBalance            float64      `json:"loan_balance"`
	ActiveLoans            []ActiveLoan `json:"active_loans"`
	IsNotificationsEnabled bool         `json:"is_notifications_enabled"`
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
	JobID string `json:"job_id,omitempty"`
}

type StatementJobStatusResponse struct {
	JobStatus   string `json:"job_status"`
	DownloadURL string `json:"download_url,omitempty"`
}

type UpdateProfileRequest struct {
	Email                *string `form:"email" binding:"omitempty,email"`
	Address              *string `form:"address"`
	RemoveProfilePicture bool    `form:"remove_profile_picture"`
}

type GetLatestAccountStatementResponse struct {
	DownloadURL string `json:"download_url"`
}
