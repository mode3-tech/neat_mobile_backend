package account

type ReportStatus string

const (
	ReportStatusPending ReportStatus = "pending"
	ReportStatusReady   ReportStatus = "ready"
	ReportStatusFailed  ReportStatus = "failed"
)

type ReportFormat string

const (
	ReportFormatCSV ReportFormat = "cvs"
	ReportFormatPDF ReportFormat = "pdf"
)

type AccountSummaryRow struct {
	ID               string
	FirstName        string
	LastName         string
	Email            string
	Phone            string
	BVN              string
	BankName         string
	AccountNumber    string
	AvailableBalance int64
	BookedBalance    int64
	InternalWalletID string
}
