package models

import "time"

const (
	NotificationTypeLoan        = "loan"
	NotificationTypeTransaction = "transaction"
	NotificationTypeSecurity    = "security"
	NotificationTypePromo       = "promo"
)

type Notification struct {
	ID        string         `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    string         `gorm:"column:user_id;type:text;not null;index"`
	Title     string         `gorm:"column:title;type:text;not null"`
	Body      string         `gorm:"column:body;type:text;not null"`
	Type      string         `gorm:"column:type;type:text;not null;check:type IN ('loan', 'transaction', 'security', 'promo')"`
	Data      map[string]any `gorm:"column:data;type:jsonb;not null;default:'{}';serializer:json"`
	IsRead    bool           `gorm:"column:is_read;not null;default:false"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	ReadAt    *time.Time     `gorm:"column:read_at;type:timestamptz"`
}

func (Notification) TableName() string {
	return "wallet_notifications"
}
