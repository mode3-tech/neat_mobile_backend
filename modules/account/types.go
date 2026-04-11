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
