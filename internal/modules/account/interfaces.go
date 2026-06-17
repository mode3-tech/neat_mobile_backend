package account

import (
	"context"
	"io"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/providers/baas"
	"time"
)

type UploadService interface {
	UploadDocument(ctx context.Context, key string, body io.ReadSeeker, contentType string) error
	UploadProfilePicture(ctx context.Context, key string, body io.ReadSeeker, contentType string) error
	PresignURL(ctx context.Context, filePath string, ttl time.Duration) (string, error)
	FileURL(key string) string
	ProfilePictureURL(key string) string
}

type DeviceVerifier interface {
	VerifyUserDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error)
}

type CustomerAccountFinder interface {
	GetCustomerDetails(ctx context.Context, customerID string) (*baas.ProvidusCustomerDetailsResponse, error)
}

type WalletFinder interface {
	GetUserWalletBalance(ctx context.Context, mobileUserID string) (*wallet.CustomerWallet, error)
}
