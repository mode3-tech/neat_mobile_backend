package accountclosure

import (
	"time"

	"neat_mobile_app_backend/internal/types"
)

type AccountClosureEventType string

const (
	AccountClosureEventTypeRequest          AccountClosureEventType = "requested"
	AccountClosureEventTypeApproval         AccountClosureEventType = "approved"
	AccountClosureEventTypeChecksStarted    AccountClosureEventType = "checks_started"
	AccountClosureEventTypeCancellation     AccountClosureEventType = "cancelled"
	AccountClosureEventTypeRejection        AccountClosureEventType = "rejected"
	AccountClosureEventTypeBlocked          AccountClosureEventType = "blocked"
	AccountClosureEventTypeUnblocked        AccountClosureEventType = "unblocked"
	AccountClosureEventProviderRequested    AccountClosureEventType = "provider_requested"
	AccountClosureEventTypeProviderApproved AccountClosureEventType = "provider_approved"
	AccountClosureEventTypeFailed           AccountClosureEventType = "failed"
	AccountClosureEventTypeClosed           AccountClosureEventType = "closed"
	AccountClosureEventTypeProcessing       AccountClosureEventType = "processing"
)

type AccountClosureStatus string

const (
	AccountClosureStatusPending    AccountClosureStatus = "pending"
	AccountClosureStatusApproved   AccountClosureStatus = "approved"
	AccountClosureStatusCompleted  AccountClosureStatus = "completed"
	AccountClosureStatusCancelled  AccountClosureStatus = "cancelled"
	AccountClosureStatusProcessing AccountClosureStatus = "processing"
	AccountClosureStatusRejected   AccountClosureStatus = "rejected"
	AccountClosureStatusClosed     AccountClosureStatus = "closed"
	AccountClosureStatusFailed     AccountClosureStatus = "failed"
	AccountClosureStatusBlocked    AccountClosureStatus = "blocked"
	AccountClosureStatusChecking   AccountClosureStatus = "checking"
)

type AccountClosureAuthorType string

const (
	AccountClosureAuthorTypeUser   AccountClosureAuthorType = "user"
	AccountClosureAuthorTypeSystem AccountClosureAuthorType = "system"
	AccountClosureAuthorTypeAdmin  AccountClosureAuthorType = "admin"
)

// AccountClosure tracks the lifecycle of a wallet account closure request.
type AccountClosure struct {
	ID                        string                   `gorm:"column:id;type:text;primaryKey" json:"id"`
	UserID                    string                   `gorm:"column:user_id;type:text;not null;index" json:"user_id"`
	ClosureReference          string                   `gorm:"column:closure_reference;type:text;not null;uniqueIndex" json:"closure_reference"`
	Status                    AccountClosureStatus     `gorm:"column:status;type:text;not null;index;check:status IN ('pending', 'approved', 'completed', 'cancelled', 'rejected', 'closed', 'failed', 'blocked', 'checking', 'processing')" json:"status"`
	RequestedAt               time.Time                `gorm:"column:requested_at;type:timestamptz;not null" json:"requested_at"`
	RequestedByType           AccountClosureAuthorType `gorm:"column:requested_by_type;type:text;not null;check:requested_by_type IN ('user', 'system', 'admin')" json:"requested_by_type"`
	RequestedByID             string                   `gorm:"column:requested_by_id;type:text;not null" json:"requested_by_id"`
	ReasonCode                *string                  `gorm:"column:reason_code;type:text;" json:"reason_code,omitempty"`
	ReasonNote                string                   `gorm:"column:reason_note;type:text;not null" json:"reason_note"`
	BlockerCode               *string                  `gorm:"column:blocker_code;type:text" json:"blocker_code,omitempty"`
	BlockerDetails            types.JSONMap            `gorm:"column:blocker_details;type:jsonb" json:"blocker_details,omitempty"`
	ApprovedByID              *string                  `gorm:"column:approved_by_id;type:text" json:"approved_by_id,omitempty"`
	ApprovedAt                *time.Time               `gorm:"column:approved_at;type:timestamptz" json:"approved_at,omitempty"`
	ProviderStatus            *string                  `gorm:"column:provider_status;type:text" json:"provider_status,omitempty"`
	InternalProviderReference *string                  `gorm:"column:internal_provider_reference;type:text" json:"internal_provider_reference,omitempty"`
	CompletedAt               *time.Time               `gorm:"column:completed_at;type:timestamptz" json:"completed_at,omitempty"`
	CancelledAt               *time.Time               `gorm:"column:cancelled_at;type:timestamptz" json:"cancelled_at,omitempty"`
	RejectedAt                *time.Time               `gorm:"column:rejected_at;type:timestamptz" json:"rejected_at,omitempty"`
	CreatedAt                 time.Time                `gorm:"column:created_at;type:timestamptz;autoCreateTime;not null" json:"created_at"`
	UpdatedAt                 time.Time                `gorm:"column:updated_at;type:timestamptz;autoUpdateTime;not null" json:"updated_at"`
}

func (AccountClosure) TableName() string {
	return "account_closures"
}

type AccountClosureEvent struct {
	ID            string                  `gorm:"primaryKey"`
	ClosureID     string                  `gorm:"column:closure_id;not null"`
	UserID        string                  `gorm:"column:user_id;not null;foreignKey:UserID"`
	EventType     AccountClosureEventType `gorm:"column:event_type;not null;check:event_type IN ('requested', 'approved', 'checks_started', 'cancelled', 'rejected', 'blocked', 'unblocked', 'processing', 'provider_requested', 'provider_approved', 'failed', 'closed')"`
	Status        AccountClosureStatus    `gorm:"column:status;not null;check:status IN ('pending', 'approved', 'rejected', 'closed', 'failed', 'checking', 'blocked', 'processing')"`
	ActorType     string                  `gorm:"column:actor_type;not null"`
	ActorID       string                  `gorm:"column:actor_id;not null"`
	ReasonNote    string                  `gorm:"column:reason_note;not null"`
	ApprovedByID  *string                 `gorm:"column:approved_by;"`
	RequestID     string                  `gorm:"column:request_id;not null"`
	IPAddress     string                  `gorm:"column:ip_address;not null"`
	DeviceID      *string                 `gorm:"column:device_id;"`
	ClosureSource string                  `gorm:"column:closure_source;not null"`
	Metadata      types.JSONMap           `gorm:"column:metadata;type:jsonb"`
	CreatedAt     time.Time               `gorm:"column:created_at;type:timestamp;autoCreateTime;not null"`
	UpdatedAt     time.Time               `gorm:"column:updated_at;type:timestamp;autoUpdateTime;not null"`
}

func (AccountClosureEvent) TableName() string {
	return "account_closure_events"
}
