package models

import "time"

type FaceCheckRecord struct {
	ID                   string    `gorm:"column:id;type:text;primaryKey"`
	VerificationRecordID string    `gorm:"column:verification_record_id;type:text;index;not null"`
	Provider             string    `gorm:"column:provider;type:text;not null"`
	Matched              bool      `gorm:"column:matched;not null"`
	Confidence           float64   `gorm:"column:confidence;not null"`
	ResponseCode         string    `gorm:"column:response_code;type:text;not null"`
	ProviderMessage      string    `gorm:"column:provider_message;type:text;not null"`
	FaceImageProvided    *string   `gorm:"column:face_image_provided;type:text"`
	ProviderReferenceID  *string   `gorm:"column:provider_reference_id;type:text;index"`
	TransactionID        *string   `gorm:"column:transaction_id;type:text;index"`
	CreatedAt            time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt            time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (FaceCheckRecord) TableName() string {
	return "wallet_face_check_records"
}
