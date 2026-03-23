package notification

import (
	"context"
	"errors"
	"neat_mobile_app_backend/models"
	"testing"
	"time"
)

type stubStore struct {
	upserted             *models.PushToken
	listTokens           []models.PushToken
	deletedByValue       []string
	createdNotifications []models.Notification
	createdTickets       []models.NotificationTicket
	pendingTickets       []models.NotificationTicket
	notifications        []models.Notification
	unreadCount          int
}

func (s *stubStore) UpsertToken(_ context.Context, row *models.PushToken) error {
	s.upserted = row
	return nil
}

func (s *stubStore) DeleteTokenByUserAndDevice(_ context.Context, _, _ string) error {
	return nil
}

func (s *stubStore) DeleteTokenByValue(_ context.Context, token string) error {
	s.deletedByValue = append(s.deletedByValue, token)
	return nil
}

func (s *stubStore) ListTokensByUserID(_ context.Context, _ string) ([]models.PushToken, error) {
	return s.listTokens, nil
}

func (s *stubStore) CreateNotification(_ context.Context, row models.Notification) error {
	s.createdNotifications = append(s.createdNotifications, row)
	return nil
}

func (s *stubStore) ListNotificationsByUserID(_ context.Context, _ string) ([]models.Notification, error) {
	return s.notifications, nil
}

func (s *stubStore) ListNotificationsPageByUserID(_ context.Context, _ string, _, _ int) ([]models.Notification, int64, error) {
	return s.notifications, int64(len(s.notifications)), nil
}

func (s *stubStore) CountUnreadByUserID(_ context.Context, _ string) (int, error) {
	return s.unreadCount, nil
}

func (s *stubStore) MarkNotificationRead(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

func (s *stubStore) MarkAllNotificationsRead(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (s *stubStore) CreateNotificationTickets(_ context.Context, rows []models.NotificationTicket) error {
	s.createdTickets = append(s.createdTickets, rows...)
	return nil
}

func (s *stubStore) ListPendingNotificationTickets(_ context.Context, _ int) ([]models.NotificationTicket, error) {
	return s.pendingTickets, nil
}

func (s *stubStore) MarkNotificationTicketReceipt(_ context.Context, _ string, _, _, _ *string, _ time.Time) error {
	return nil
}

type stubSender struct {
	tickets []ExpoPushTicket
	err     error
}

func (s *stubSender) Send(_ context.Context, _ []ExpoPushMessage) ([]ExpoPushTicket, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.tickets, nil
}

func (s *stubSender) GetReceipts(_ context.Context, _ []string) (map[string]ExpoPushReceipt, error) {
	return map[string]ExpoPushReceipt{}, nil
}

func TestRegisterTokenRejectsInvalidExpoPushToken(t *testing.T) {
	service := NewService(&stubStore{}, nil, "default")

	err := service.RegisterToken(context.Background(), "user-1", RegisterTokenRequest{
		ExpoPushToken: "not-a-real-token",
		DeviceID:      "device-1",
		Platform:      "android",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "invalid expo push token" {
		t.Fatalf("expected invalid expo push token, got %q", got)
	}
}

func TestSendToUserDeletesDeviceNotRegisteredTokens(t *testing.T) {
	store := &stubStore{
		listTokens: []models.PushToken{
			{UserID: "user-1", DeviceID: "device-1", ExpoPushToken: "ExpoPushToken[token-1]", Platform: "android"},
			{UserID: "user-1", DeviceID: "device-2", ExpoPushToken: "ExpoPushToken[token-2]", Platform: "ios"},
		},
	}
	sender := &stubSender{
		tickets: []ExpoPushTicket{
			{Status: "ok"},
			{Status: "error", Details: map[string]interface{}{"error": "DeviceNotRegistered"}},
		},
	}
	service := NewService(store, sender, "default")

	err := service.SendToUser(context.Background(), "user-1", "Loan Approved!", models.NotificationTypeLoan, "Your loan has been approved.", map[string]any{
		"screen": "/(loan)/details",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(store.createdNotifications) != 1 {
		t.Fatalf("expected 1 stored notification, got %d", len(store.createdNotifications))
	}

	if len(store.deletedByValue) != 1 {
		t.Fatalf("expected 1 deleted token, got %d", len(store.deletedByValue))
	}
	if got := store.deletedByValue[0]; got != "ExpoPushToken[token-2]" {
		t.Fatalf("expected token-2 to be deleted, got %q", got)
	}
}

func TestSendToUserReturnsSenderError(t *testing.T) {
	expectedErr := errors.New("expo down")
	service := NewService(&stubStore{
		listTokens: []models.PushToken{
			{UserID: "user-1", DeviceID: "device-1", ExpoPushToken: "ExpoPushToken[token-1]", Platform: "android"},
		},
	}, &stubSender{err: expectedErr}, "default")

	err := service.SendToUser(context.Background(), "user-1", "Title", models.NotificationTypeTransaction, "Body", nil)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}
