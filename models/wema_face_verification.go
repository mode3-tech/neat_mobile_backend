package models

import "time"

// Outcomes of Wema's face biometric web app, as reported to our
// facial-verification callback.
const (
	WemaFaceStatusVerified = "verified"
	WemaFaceStatusFailed   = "failed"
)

// WemaFaceVerification is a result from Wema's face biometric web app,
// delivered to our facial-verification callback. For a verified result,
// CorrelationID is Wema's own - it is what ALAT's tier 2 account creation
// requires. A failed one carries Wema's error text. The BVN/NIN is stored only
// as a hash.
type WemaFaceVerification struct {
	ID            string    `gorm:"column:id;type:text;primaryKey"`
	IDType        string    `gorm:"column:id_type;type:text;not null;index:idx_wema_face_subject,priority:1"`
	SubjectHash   string    `gorm:"column:subject_hash;type:text;not null;index:idx_wema_face_subject,priority:2"`
	CorrelationID string    `gorm:"column:correlation_id;type:text;not null"`
	Status        string    `gorm:"column:status;type:text;not null;default:'verified'"`
	ErrorMessage  string    `gorm:"column:error_message;type:text;not null;default:''"`
	ExpiresAt     time.Time `gorm:"column:expires_at;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (WemaFaceVerification) TableName() string {
	return "wallet_wema_face_verifications"
}
