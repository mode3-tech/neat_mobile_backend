package notification

import (
	"context"
	"errors"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/device"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store interface {
	UpsertToken(ctx context.Context, row *models.PushToken) error
	IsNotificationsEnabled(ctx context.Context, userID string) (bool, error)
	DeleteTokenByUserAndDevice(ctx context.Context, userID, deviceID string) error
	DeleteTokenByValue(ctx context.Context, token string) error
	ListTokensByUserID(ctx context.Context, userID string) ([]models.PushToken, error)
	CreateNotification(ctx context.Context, row models.Notification) error
	ListNotificationsPageByUserID(ctx context.Context, userID string, limit, offset int) ([]models.Notification, int64, error)
	CountUnreadByUserID(ctx context.Context, userID string) (int, error)
	MarkNotificationRead(ctx context.Context, userID, notificationID string) (bool, error)
	MarkAllNotificationsRead(ctx context.Context, userID string) (int64, error)
	CreateNotificationTickets(ctx context.Context, rows []models.NotificationTicket) error
	ListPendingNotificationTickets(ctx context.Context, limit int) ([]models.NotificationTicket, error)
	MarkNotificationTicketReceipt(ctx context.Context, expoTicketID string, receiptStatus, receiptMessage, receiptError *string, checkedAt time.Time) error
	TogglePushNotifications(ctx context.Context, mobileUserID string) (bool, error)
	IsVerifiedDevice(ctx context.Context, mobileUserID string, deviceID string) bool
	FindDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error)
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) IsNotificationsEnabled(ctx context.Context, mobileUserID string) (bool, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return false, errors.New("user id is required")
	}

	var user models.User
	if err := r.db.WithContext(ctx).Model(&models.User{}).Select("is_notifications_enabled").Where("id = ?", mobileUserID).First(&user).Error; err != nil {
		return false, err
	}

	if user.IsNotificationsEnabled == nil {
		return false, nil
	}

	return *user.IsNotificationsEnabled, nil
}

func (r *Repository) UpsertToken(ctx context.Context, row *models.PushToken) error {
	if row == nil {
		return errors.New("push token row is required")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("expo_push_token = ? AND (user_id <> ? OR device_id <> ?)", row.ExpoPushToken, row.UserID, row.DeviceID).
			Delete(&models.PushToken{}).Error; err != nil {
			return err
		}

		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "device_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"expo_push_token": row.ExpoPushToken,
				"platform":        row.Platform,
				"updated_at":      time.Now().UTC(),
			}),
		}).Create(row).Error
	})
}

func (r *Repository) DeleteTokenByUserAndDevice(ctx context.Context, userID, deviceID string) error {
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	if userID == "" || deviceID == "" {
		return nil
	}

	return r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		Delete(&models.PushToken{}).Error
}

func (r *Repository) DeleteTokenByValue(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}

	return r.db.WithContext(ctx).
		Where("expo_push_token = ?", token).
		Delete(&models.PushToken{}).Error
}

func (r *Repository) ListTokensByUserID(ctx context.Context, userID string) ([]models.PushToken, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("user id is required")
	}

	var rows []models.PushToken
	if err := r.db.WithContext(ctx).
		Model(&models.PushToken{}).
		Where("user_id = ?", userID).
		Order("updated_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *Repository) CreateNotification(ctx context.Context, row models.Notification) error {
	row.UserID = strings.TrimSpace(row.UserID)
	row.Title = strings.TrimSpace(row.Title)
	row.Body = strings.TrimSpace(row.Body)
	row.Type = strings.TrimSpace(strings.ToLower(row.Type))
	if row.UserID == "" {
		return errors.New("user id is required")
	}
	if row.Title == "" {
		return errors.New("title is required")
	}
	if row.Body == "" {
		return errors.New("body is required")
	}
	if !isSupportedNotificationType(row.Type) {
		return errors.New("notification type is invalid")
	}
	if row.Data == nil {
		row.Data = map[string]any{}
	}

	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) ListNotificationsPageByUserID(ctx context.Context, userID string, limit, offset int) ([]models.Notification, int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, 0, errors.New("user id is required")
	}

	baseQuery := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ?", userID)

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var notifications []models.Notification
	if err := baseQuery.
		Order("created_at DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error; err != nil {
		return nil, 0, err
	}

	return notifications, total, nil
}

func (r *Repository) CountUnreadByUserID(ctx context.Context, userID string) (int, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, errors.New("user id is required")
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return int(count), nil
}

func (r *Repository) MarkNotificationRead(ctx context.Context, userID, notificationID string) (bool, error) {
	userID = strings.TrimSpace(userID)
	notificationID = strings.TrimSpace(notificationID)
	if userID == "" {
		return false, errors.New("user id is required")
	}
	if notificationID == "" {
		return false, errors.New("notification id is required")
	}

	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND id = ? AND is_read = ?", userID, notificationID, false).
		Updates(map[string]any{
			"is_read": true,
			"read_at": &now,
		})
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

func (r *Repository) MarkAllNotificationsRead(ctx context.Context, userID string) (int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, errors.New("user id is required")
	}

	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]any{
			"is_read": true,
			"read_at": &now,
		})
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

func isSupportedNotificationType(notificationType string) bool {
	switch strings.TrimSpace(strings.ToLower(notificationType)) {
	case models.NotificationTypeLoan,
		models.NotificationTypeTransaction,
		models.NotificationTypeSecurity,
		models.NotificationTypePromo:
		return true
	default:
		return false
	}
}

func (r *Repository) CreateNotificationTickets(ctx context.Context, rows []models.NotificationTicket) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}

func (r *Repository) ListPendingNotificationTickets(ctx context.Context, limit int) ([]models.NotificationTicket, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows []models.NotificationTicket
	if err := r.db.WithContext(ctx).
		Model(&models.NotificationTicket{}).
		Where("receipt_checked_at IS NULL").
		Order("created_at ASC, id ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *Repository) MarkNotificationTicketReceipt(ctx context.Context, expoTicketID string, receiptStatus, receiptMessage, receiptError *string, checkedAt time.Time) error {
	expoTicketID = strings.TrimSpace(expoTicketID)
	if expoTicketID == "" {
		return errors.New("expo ticket id is required")
	}

	updates := map[string]any{
		"receipt_checked_at": checkedAt.UTC(),
		"receipt_status":     trimmedStringPtr(receiptStatus),
		"receipt_message":    trimmedStringPtr(receiptMessage),
		"receipt_error":      trimmedStringPtr(receiptError),
	}

	return r.db.WithContext(ctx).
		Model(&models.NotificationTicket{}).
		Where("expo_ticket_id = ?", expoTicketID).
		Updates(updates).Error
}

func trimmedStringPtr(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func (r *Repository) TogglePushNotifications(ctx context.Context, mobileUserID string) (bool, error) {
	if err := r.db.WithContext(ctx).
		Model(models.User{}).
		Where("id = ?", mobileUserID).
		Update("is_notifications_enabled", gorm.Expr("NOT is_notifications_enabled")).Error; err != nil {
		return false, err
	}

	var user models.User
	if err := r.db.WithContext(ctx).
		Select("is_notifications_enabled").
		Where("id = ?", mobileUserID).
		First(&user).Error; err != nil {
		return false, err
	}

	return *user.IsNotificationsEnabled, nil
}

func (r *Repository) IsVerifiedDevice(ctx context.Context, mobileUserID string, deviceID string) bool {
	err := r.db.WithContext(ctx).
		Model(device.UserDevice{}).
		Where("user_id = ? AND device_id = ? AND is_trusted = ? AND is_active = ?", mobileUserID, deviceID, true, true).Error
	if err != nil {
		return false
	}
	return true
}

func (r *Repository) FindDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error) {
	var result device.UserDevice
	if err := r.db.WithContext(ctx).
		Model(&device.UserDevice{}).
		Select("*").Where("user_id = ? AND device_id = ?", mobileUserID, deviceID).
		First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}
