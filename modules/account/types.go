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
	ID                     string
	ProfilePicture         string
	FirstName              string
	LastName               string
	DOB                    time.Time
	Email                  string
	Phone                  string
	BVN                    string
	BankName               string
	Address                string
	AccountNumber          string
	AvailableBalance       int64
	BookedBalance          int64
	InternalWalletID       string
	IsNotificationsEnabled bool
	CoreCustomerID         *string
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
