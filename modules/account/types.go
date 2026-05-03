package account

import "time"

type ReportStatus string

const (
	ReportStatusPending    ReportStatus = "pending"
	ReportStatusProcessing ReportStatus = "processing"
	ReportStatusReady      ReportStatus = "ready"
	ReportStatusFailed     ReportStatus = "failed"
)

type ReportFormat string

const (
	ReportFormatCSV ReportFormat = "xlsx"
	ReportFormatPDF ReportFormat = "pdf"
)

type AccountSummaryRow struct {
	ID                     string    `gorm:"id"`
	ProfilePicture         string    `gorm:"profile_picture"`
	FirstName              string    `gorm:"first_name"`
	LastName               string    `gorm:"last_name"`
	DOB                    time.Time `gorm:"dob"`
	Email                  string    `gorm:"email"`
	Phone                  string    `gorm:"phone"`
	BVN                    string    `gorm:"bvn"`
	BankName               string    `gorm:"bank_name"`
	Address                string    `gorm:"address"`
	AccountNumber          string    `gorm:"account_number"`
	AvailableBalance       int64     `gorm:"available_balance"`
	BookedBalance          int64     `gorm:"booked_balance"`
	InternalWalletID       string    `gorm:"internal_wallet_id"`
	IsNotificationsEnabled bool      `gorm:"is_notifications_enabled"`
	CoreCustomerID         *string   `gorm:"core_customer_id"`
}

type DashboardLoanItem struct {
	LoanID             string  `gorm:"column:loan_id"`
	LoanNumber         string  `gorm:"column:loan_number"`
	LoanAmount         float64 `gorm:"column:loan_amount"`
	OutstandingBalance float64 `gorm:"column:outstanding_balance"`
	NextPayment        float64 `gorm:"column:next_payment"`
	NextDueDate        string  `gorm:"column:next_due_date"`
	Status             string  `gorm:"column:status"`
}

type UpdateProfileData struct {
	ProfilePictureURL *string
	Email             *string
	Address           *string
}

type statementTxRow struct {
	Date        string
	Description string
	Reference   string
	Debit       string
	Credit      string
	Balance     string
}

type statementTemplateData struct {
	TodayDate        string
	StartDate        string
	EndDate          string
	AccountName      string
	Address          string
	AccountNumber    string
	OpeningBalance   string
	TotalWithdrawals string
	TotalLodgement   string
	ClosingBalance   string
	Transactions     []statementTxRow
}
