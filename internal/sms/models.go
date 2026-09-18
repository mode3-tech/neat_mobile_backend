package sms

import "time"

type OutgoingSMSStatus string

const (
	OutgoingSMSStatusPending OutgoingSMSStatus = "pending"
	OutgoingSMSStatusSent    OutgoingSMSStatus = "successful"
	OutgoingSMSStatusFailed  OutgoingSMSStatus = "failed"
)

type OutgoingSMS struct {
	ID               string            `gorm:"column:id;primaryKey"`
	Phone            string            `gorm:"column:phone;index;type:text"`
	Recepient        string            `gorm:"column:recepient;type:text;not null;"`
	ReasonForFailure string            `gorm:"column:reason_for_failure;type:text"`
	RetryCount       int               `gorm:"column:retry_count;type:int;not null;default:0"`
	Message          string            `gorm:"column:message;type:text"`
	Status           OutgoingSMSStatus `gorm:"column:status;not null;default:pending;check:status IN ('pending', 'successful', 'failed')"`
	LastRetryAt      *time.Time        `gorm:"column:last_attempt_at;type:timestamptz"`
	CreatedAt        time.Time         `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime;default:now()"`
	SentAt           *time.Time        `gorm:"column:sent_at;type:timestamptz"`
	UpdatedAt        *time.Time        `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (OutgoingSMS) TableName() string {
	return "wallet_outgoing_sms"
}
