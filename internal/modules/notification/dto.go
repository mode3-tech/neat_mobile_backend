package notification

import (
	"neat_mobile_app_backend/models"
	"time"
)

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
	Message   string `json:"message"`
	IsEnabled bool   `json:"is_enabled"`
}

type UnreadNotificationCountResponse struct {
	Count int `json:"count"`
}

type NotificationParams struct {
	ID string `uri:"id" binding:"required"`
}

type MarkNotificationReadResponse struct {
	Updated *bool `json:"bool"`
}

type NotificationDTO struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Type      string         `json:"type"`
	Data      map[string]any `json:"data"`
	IsRead    bool           `json:"is_read"`
	CreatedAt time.Time      `json:"created_at"`
	ReadAt    *time.Time     `json:"read_at"`
}

type GetNotificationsResult struct {
	Notifications []NotificationDTO
	Page          int
	Limit         int
	Total         int64
}

func notificationToDTO(n models.Notification) NotificationDTO {
	return NotificationDTO{
		ID:        n.ID,
		UserID:    n.UserID,
		Title:     n.Title,
		Body:      n.Body,
		Type:      n.Type,
		Data:      n.Data,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
		ReadAt:    n.ReadAt,
	}
}
