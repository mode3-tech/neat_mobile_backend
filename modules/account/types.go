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
}

type UpdateProfileData struct {
	ProfilePictureURL *string
	Email             *string
	Address           *string
}
