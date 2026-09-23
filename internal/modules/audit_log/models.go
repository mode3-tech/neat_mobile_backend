package auditlog

import (
	"neat_mobile_app_backend/internal/types"
	"time"
)

type AuditLog struct {
	ID           string        `gorm:"primaryKey"`
	Timestamp    time.Time     `gorm:"index;not null"`
	ActorType    string        `gorm:"not null"`
	ActorID      string        `gorm:"index;not null"`
	Action       string        `gorm:"index;not null"`
	ResourceType ResourceType  `gorm:"index:idx_audit_resource;check:resource_type IN ('wallet', 'user', 'logout', 'login', 'refresh_token', 'user_account', 'transaction', 'loan', 'loan_application', 'tier_upgrade', 'device', 'session', 'nin', 'bvn', 'beneficiary', 'card', 'savings_goal')"`
	ResourceID   string        `gorm:"index:idx_audit_resource;not null"`
	Status       LogStatus     `gorm:"not null"`
	RequestID    string        `gorm:"index;not null"`
	IPAddress    string        `gorm:"not null"`
	Metadata     types.JSONMap `gorm:"type:jsonb;"`
}

func (AuditLog) TableName() string {
	return "wallet_audit_logs"
}
