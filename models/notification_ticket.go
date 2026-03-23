package models

import "time"

type NotificationTicket struct {
	ID               string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey"`
	NotificationID   string     `gorm:"column:notification_id;type:uuid;not null;index"`
	UserID           string     `gorm:"column:user_id;type:text;not null;index"`
	ExpoPushToken    string     `gorm:"column:expo_push_token;type:text;not null"`
	ExpoTicketID     string     `gorm:"column:expo_ticket_id;type:text;not null;uniqueIndex"`
	ReceiptStatus    *string    `gorm:"column:receipt_status;type:text"`
	ReceiptMessage   *string    `gorm:"column:receipt_message;type:text"`
	ReceiptError     *string    `gorm:"column:receipt_error;type:text"`
	ReceiptCheckedAt *time.Time `gorm:"column:receipt_checked_at;type:timestamptz;index"`
	CreatedAt        time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
}

func (NotificationTicket) TableName() string {
	return "wallet_notification_tickets"
}
