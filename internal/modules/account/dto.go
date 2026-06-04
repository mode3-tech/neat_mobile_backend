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

type AccountLimitResponse struct {
	ActivationCap ActivationCap `json:"activation_cap"`
	Outflow       Outflow       `json:"out_flow"`
	Inflow        Inflow        `json:"in_flow"`
}

type ActivationCap struct {
	Active    bool      `json:"active"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	CapAmount int64     `json:"cap_amount,omitempty"`
	Currency  string    `json:"currency,omitempty"`
}

type Outflow struct {
	Limit     int64 `json:"limit,omitempty"`
	Spent     int64 `json:"spent,omitempty"`
	Remaining int64 `json:"remaining,omitempty"`
}

type Inflow struct {
	Capped    bool  `json:"capped"`
	Limit     int64 `json:"limit"`
	Remaining int64 `json:"remaining"`
}

// {

//   "data": {
//     "activation_cap": {
//       "active": true,
//       "expires_at": "2026-06-03T15:33:00Z",   // ISO 8601, UTC — app formats to "3:33 PM tomorrow"
//       "cap_amount": 2000000,                    // ₦20,000 in kobo (see note below)
//       "currency": "NGN",

//       "outflow": {
//         "limit": 2000000,
//         "spent": 1500000,                       // already sent in the window
//         "remaining": 500000                     // app validates against THIS, not cap_amount
//       },

//       "inflow": {
//         "capped": true,                         // true for NEW holders, false for existing
//         "limit": 2000000,
//         "remaining": 2000000
//       },

//       "fees_count_toward_cap": false            // does the ₦10.75 commission eat the allowance?
//     }
//   }
// }
