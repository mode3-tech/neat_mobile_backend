package models

import "time"

// WemaFaceVerification is a successful result from Wema's face biometric web
// app, delivered to our facial-verification callback. CorrelationID is Wema's
// own - it is what ALAT's tier 2 account creation requires. The BVN/NIN is
// stored only as a hash.
type WemaFaceVerification struct {
	ID            string    `gorm:"column:id;type:text;primaryKey"`
	IDType        string    `gorm:"column:id_type;type:text;not null;index:idx_wema_face_subject,priority:1"`
	SubjectHash   string    `gorm:"column:subject_hash;type:text;not null;index:idx_wema_face_subject,priority:2"`
	CorrelationID string    `gorm:"column:correlation_id;type:text;not null"`
	ExpiresAt     time.Time `gorm:"column:expires_at;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (WemaFaceVerification) TableName() string {
	return "wallet_wema_face_verifications"
}
