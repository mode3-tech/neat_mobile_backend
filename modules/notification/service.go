package notification

import (
	"context"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/models"
	"strings"

	"github.com/google/uuid"
)

const (
	defaultNotificationPage     = 1
	defaultNotificationPageSize = 20
	maxNotificationPageSize     = 100
)

type Sender interface {
	Send(ctx context.Context, messages []ExpoPushMessage) ([]ExpoPushTicket, error)
	GetReceipts(ctx context.Context, receiptIDs []string) (map[string]ExpoPushReceipt, error)
}

type Service struct {
	repo             Store
	sender           Sender
	defaultChannelID string
}

var ErrSenderNotConfigured = errors.New("push sender is not configured")

func NewService(repo Store, sender Sender, defaultChannelID string) *Service {
	return &Service{
		repo:             repo,
		sender:           sender,
		defaultChannelID: strings.TrimSpace(defaultChannelID),
	}
}

func (s *Service) RegisterToken(ctx context.Context, userID string, req RegisterTokenRequest) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id is required")
	}
	if s.repo == nil {
		return errors.New("notification repository is not configured")
	}

	deviceID := strings.TrimSpace(req.DeviceID)
	expoPushToken := strings.TrimSpace(req.ExpoPushToken)
	platform := strings.TrimSpace(strings.ToLower(req.Platform))

	if deviceID == "" {
		return errors.New("device id is required")
	}
	if expoPushToken == "" {
		return errors.New("expo push token is required")
	}
	if !isSupportedPlatform(platform) {
		return errors.New("platform must be ios or android")
	}
	if !looksLikeExpoPushToken(expoPushToken) {
		return errors.New("invalid expo push token")
	}

	row := &models.PushToken{
		ID:            uuid.NewString(),
		UserID:        userID,
		DeviceID:      deviceID,
		ExpoPushToken: expoPushToken,
		Platform:      platform,
	}

	return s.repo.UpsertToken(ctx, row)
}

func (s *Service) DeleteToken(ctx context.Context, userID, deviceID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id is required")
	}
	if s.repo == nil {
		return errors.New("notification repository is not configured")
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("device id is required")
	}

	return s.repo.DeleteTokenByUserAndDevice(ctx, userID, deviceID)
}

func (s *Service) SendToUser(ctx context.Context, userID, title, typ, body string, data map[string]any) error {
	return s.SendToUserWithOptions(ctx, SendNotificationRequest{
		UserID: userID,
		Title:  title,
		Type:   typ,
		Body:   body,
		Data:   data,
	})
}

func (s *Service) SendToUserWithOptions(ctx context.Context, req SendNotificationRequest) error {
	if s.repo == nil {
		return errors.New("notification repository is not configured")
	}
	if s.sender == nil {
		return ErrSenderNotConfigured
	}

	var channel string

	switch req.Type {
	case "loan", "transaction", "promo":
		channel = "transactions"
	case "security":
		channel = "security"
	}

	userID := strings.TrimSpace(req.UserID)
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Body)
	sound := strings.TrimSpace(req.Sound)
	channelID := strings.TrimSpace(req.ChannelID)

	if userID == "" {
		return errors.New("user id is required")
	}
	if title == "" {
		return errors.New("title is required")
	}
	if body == "" {
		return errors.New("body is required")
	}
	if sound == "" {
		sound = "default"
	}
	if channelID == "" {
		channelID = strings.TrimSpace(channel)
	}
	if channelID == "" {
		channelID = s.defaultChannelID
	}

	notification := &models.Notification{
		ID:     uuid.NewString(),
		UserID: userID,
		Title:  title,
		Body:   body,
		Type:   req.Type,
		Data:   req.Data,
	}

	if err := s.repo.CreateNotification(ctx, *notification); err != nil {
		return err
	}

	tokens, err := s.repo.ListTokensByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	messages := make([]ExpoPushMessage, 0, len(tokens))
	for _, token := range tokens {
		messages = append(messages, ExpoPushMessage{
			To:        token.ExpoPushToken,
			Title:     title,
			Body:      body,
			Data:      req.Data,
			Sound:     sound,
			ChannelID: channelID,
		})
	}

	tickets, err := s.sender.Send(ctx, messages)
	if err != nil {
		return err
	}
	ticketRows := make([]models.NotificationTicket, 0, len(tickets))

	for i, ticket := range tickets {
		if i >= len(tokens) {
			break
		}

		if isDeviceNotRegistered(ticket) {
			if delErr := s.repo.DeleteTokenByValue(ctx, tokens[i].ExpoPushToken); delErr != nil {
				log.Printf("notification: failed to delete unregistered push token for user %s: %v", userID, delErr)
			}
			continue
		}

		if strings.EqualFold(strings.TrimSpace(ticket.Status), "ok") && strings.TrimSpace(ticket.ID) != "" {
			ticketRows = append(ticketRows, models.NotificationTicket{
				ID:             uuid.NewString(),
				NotificationID: notification.ID,
				UserID:         userID,
				ExpoPushToken:  tokens[i].ExpoPushToken,
				ExpoTicketID:   strings.TrimSpace(ticket.ID),
			})
		}
	}

	if err := s.repo.CreateNotificationTickets(ctx, ticketRows); err != nil {
		log.Printf("notification: failed to persist expo ticket rows for notification %s: %v", notification.ID, err)
	}
	return nil
}

func looksLikeExpoPushToken(token string) bool {
	token = strings.TrimSpace(token)
	return strings.HasPrefix(token, "ExponentPushToken[") || strings.HasPrefix(token, "ExpoPushToken[")
}

func isSupportedPlatform(platform string) bool {
	switch strings.TrimSpace(strings.ToLower(platform)) {
	case "ios", "android":
		return true
	default:
		return false
	}
}

func isDeviceNotRegistered(ticket ExpoPushTicket) bool {
	if !strings.EqualFold(strings.TrimSpace(ticket.Status), "error") {
		return false
	}

	if strings.EqualFold(strings.TrimSpace(ticket.Message), "DeviceNotRegistered") {
		return true
	}

	if ticket.Details == nil {
		return false
	}

	value, ok := ticket.Details["error"]
	if !ok {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "DeviceNotRegistered")
}

func (s *Service) StoreNotification(ctx context.Context, req SendNotificationRequest) error {
	if s.repo == nil {
		return errors.New("notification repository is not configured")
	}

	userID := strings.TrimSpace(req.UserID)
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Body)

	if userID == "" {
		return errors.New("user id is required")
	}
	if title == "" {
		return errors.New("title is required")
	}
	if body == "" {
		return errors.New("body is required")
	}

	row := models.Notification{
		ID:     uuid.NewString(),
		UserID: userID,
		Title:  title,
		Body:   body,
		Type:   req.Type,
		Data:   req.Data,
	}

	return s.repo.CreateNotification(ctx, row)
}

func (s *Service) GetNotifications(ctx context.Context, userID string, page, pageSize int) (*ListNotificationsResponse, error) {
	if s.repo == nil {
		return nil, errors.New("notification repository is not configured")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("user id is required")
	}

	page, pageSize, offset := normalizeNotificationPagination(page, pageSize)

	notifications, total, err := s.repo.ListNotificationsPageByUserID(ctx, userID, pageSize, offset)
	if err != nil {
		return nil, err
	}

	return &ListNotificationsResponse{
		Notifications: notifications,
		Page:          page,
		PageSize:      pageSize,
		Total:         total,
		HasNext:       int64(offset+len(notifications)) < total,
	}, nil
}

func (s *Service) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	if s.repo == nil {
		return 0, errors.New("notification repository is not configured")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, errors.New("user id is required")
	}

	return s.repo.CountUnreadByUserID(ctx, userID)
}

func (s *Service) MarkNotificationRead(ctx context.Context, userID, notificationID string) (bool, error) {
	if s.repo == nil {
		return false, errors.New("notification repository is not configured")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, errors.New("user id is required")
	}

	return s.repo.MarkNotificationRead(ctx, userID, notificationID)
}

func (s *Service) MarkAllNotificationsRead(ctx context.Context, userID string) (int64, error) {
	if s.repo == nil {
		return 0, errors.New("notification repository is not configured")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, errors.New("user id is required")
	}

	return s.repo.MarkAllNotificationsRead(ctx, userID)
}
