package notification

import "neat_mobile_app_backend/models"

type RegisterTokenRequest struct {
	ExpoPushToken string `json:"expo_push_token" binding:"required"`
	DeviceID      string `json:"device_id" binding:"required"`
	Platform      string `json:"platform" binding:"required,oneof=ios android"`
}

type SendNotificationRequest struct {
	UserID    string         `json:"user_id" binding:"required"`
	Title     string         `json:"title" binding:"required"`
	Body      string         `json:"body" binding:"required"`
	Type      string         `json:"type" binding:"required,oneof=loan transaction security promo"`
	Data      map[string]any `json:"data,omitempty"`
	Sound     string         `json:"sound,omitempty"`
	ChannelID string         `json:"channel_id,omitempty"`
}

type ExpoPushMessage struct {
	To        string         `json:"to"`
	Title     string         `json:"title,omitempty"`
	Body      string         `json:"body,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Sound     string         `json:"sound,omitempty"`
	ChannelID string         `json:"channelId,omitempty"`
}

type ExpoPushTicket struct {
	ID      string                 `json:"id,omitempty"`
	Status  string                 `json:"status"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type ExpoPushReceipt struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type ListNotificationsQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

type ListNotificationsResponse struct {
	Notifications []models.Notification `json:"notifications"`
	Page          int                   `json:"page"`
	PageSize      int                   `json:"page_size"`
	Total         int64                 `json:"total"`
	HasNext       bool                  `json:"has_next"`
}

type TogglePushNotificationsResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	IsEnabled bool   `json:"is_enabled"`
}
