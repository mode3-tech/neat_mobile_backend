package wallet

import (
	"context"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/sms"
	"time"
)

type BankResponse struct {
	Status bool   `json:"status"`
	Banks  []Bank `json:"banks"`
}

type Bank struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type BankDetailsResponse struct {
	Status  bool        `json:"status"`
	Account BankDetails `json:"account"`
}

type BankDetails struct {
	BankCode      string `json:"bankCode"`
	AccountName   string `json:"accountName"`
	AccountNumber string `json:"accountNumber"`
}

type ProvidusService interface {
	FetchBanks(ctx context.Context) ([]Bank, error)
	FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*BankDetails, error)
	GetCustomerDetails(ctx context.Context, customerID string) (*ProvidusCustomerDetailsResponse, error)
	InitiateTransfer(ctx context.Context, providusCustomerID string, transferInfo *TransferRequest) (*TransferResponse, error)
}

type DeviceVerifier interface {
	VerifyUserDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error)
}

type SmsSender interface {
	Send(ctx context.Context, destination, message string) error
}

type NotificationSender interface {
	SendToUser(ctx context.Context, userID, title, typ, transactionID, body string, data map[string]any) error
}

type OutgoingSMSService interface {
	CreateOutgoingSMS(ctx context.Context, id, phone, message, recipient string) (*sms.OutgoingSMS, error)
	UpdateOutgoingSMS(ctx context.Context, id string, status sms.OutgoingSMSStatus, sentAt *time.Time, reasonForFailure string) error
	GetPendingOutgoingSMS(ctx context.Context, retryBackoff time.Duration) ([]sms.OutgoingSMS, error)
}
