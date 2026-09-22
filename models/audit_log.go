package models

import (
	"time"
)

type ResourceType string

const (
	ResourceTypeWallet          ResourceType = "wallet"
	ResourceTypeUser            ResourceType = "user"
	ResourceTypeUserAccount     ResourceType = "user_account"
	ResourceTypeTransaction     ResourceType = "transaction"
	ResourceTypeLoan            ResourceType = "loan"
	ResourceTypeLoanApplication ResourceType = "loan_application"
	ResourceTypeTierUpgrade     ResourceType = "tier_upgrade"
	ResourceTypeDevice          ResourceType = "device"
	ResourceTypeSession         ResourceType = "session"
)

type AuditLog struct {
	ID           int64        `gorm:"primaryKey;autoIncrement"`
	Timestamp    time.Time    `gorm:"index"`
	ActorType    string       `gorm:""`
	ActorID      string       `gorm:"index"`
	Action       string       `gorm:"index"`
	ResourceType ResourceType `gorm:"index:idx_audit_resource;check:resource_type IN ('wallet', 'user', 'user_account', 'transaction', 'loan', 'loan_application', 'tier_upgrade', 'device', 'session')"`
	ResourceID   string       `gorm:"index:idx_audit_resource"`
	Status       string       `gorm:""`
	RequestID    string       `gorm:"index"`
	IPAddress    string       `gorm:""`
	Metadata     map[string]interface{}
}

func (AuditLog) TableName() string {
	return "wallet_audit_logs"
}
