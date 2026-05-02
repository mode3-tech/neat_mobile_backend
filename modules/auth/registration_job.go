package auth

import "time"

type RegistrationJobStatus string

const (
	RegistrationJobStatusPending    RegistrationJobStatus = "pending"
	RegistrationJobStatusProcessing RegistrationJobStatus = "processing"
	RegistrationJobStatusCompleted  RegistrationJobStatus = "completed"
	RegistrationJobStatusFailed     RegistrationJobStatus = "failed"
)

type RegistrationJob struct {
	ID                    string                `gorm:"column:id;type:text;primaryKey"`
	IdempotencyKey        string                `gorm:"column:idempotency_key;type:text;not null;uniqueIndex"`
	MobileUserID          string                `gorm:"column:mobile_user_id;type:text;not null;uniqueIndex"`
	InternalWalletID      string                `gorm:"column:internal_wallet_id;type:text;not null;uniqueIndex"`
	Phone                 string                `gorm:"column:phone;type:text;not null;index"`
	Status                RegistrationJobStatus `gorm:"column:status;type:text;not null;default:'pending';index"`
	SnapshotJSON          string                `gorm:"column:snapshot_json;type:text;not null"`
	WalletResponseJSON    *string               `gorm:"column:wallet_response_json;type:text"`
	SessionClaimTokenHash *string               `gorm:"column:session_claim_token_hash;type:text"`
	SessionClaimExpiresAt *time.Time            `gorm:"column:session_claim_expires_at;type:timestamptz"`
	SessionClaimedAt      *time.Time            `gorm:"column:session_claimed_at;type:timestamptz"`
	LastError             *string               `gorm:"column:last_error;type:text"`
	Attempts              int                   `gorm:"column:attempts;not null;default:0"`
	CompletedAt           *time.Time            `gorm:"column:completed_at;type:timestamptz"`
	CreatedAt             time.Time             `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt             *time.Time            `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (RegistrationJob) TableName() string {
	return "wallet_registration_jobs"
}
