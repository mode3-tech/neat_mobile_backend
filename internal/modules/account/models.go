package account

import "time"

type AccountReportJob struct {
	ID           string       `gorm:"column:id;type:text;primaryKey"`
	MobileUserID string       `gorm:"column:mobile_user_id;type:text;not null;index"`
	WalletID     string       `gorm:"column:wallet_id;type:text;not null;index"`
	Type         string       `gorm:"column:type;not null;default:'account_statement'"`
	Status       ReportStatus `gorm:"column:status;not null;default:'pending'"`
	FilePath     string       `gorm:"column:file_path"`
	DownloadURL  string       `gorm:"column:download_url"`
	URLExpiresAt *time.Time   `gorm:"column:url_expires_at;type:timestamptz"`
	DateFrom     *time.Time   `gorm:"column:date_from;type:timestamptz"`
	DateTo       *time.Time   `gorm:"column:date_to"`
	Format       ReportFormat `gorm:"column:format"`
	ErrorMsg     *string      `gorm:"column:error_msg"`
	CreatedAt    time.Time    `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt    *time.Time   `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (AccountReportJob) TableName() string {
	return "wallet_account_report_jobs"
}
