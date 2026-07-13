package accountclosure

import (
	"context"
	"neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/providers/baas"
)

type XpressWalletService interface {
	GetCustomerDetails(ctx context.Context, customerID string) (*baas.ProvidusCustomerDetailsResponse, error)
	CloseWallet(ctx context.Context, customerID string) error
}

type LoanService interface {
	HasActiveLoans(ctx context.Context, mobileUserID string) (bool, error)
}

type WalletService interface {
	GetUserWalletBalance(ctx context.Context, mobileUserID string) (*wallet.CustomerWallet, error)
}

type DeviceService interface {
	DeactivateDevice(ctx context.Context, mobileUserID, deviceID string) error
}

type UserService interface {
	GetUserDetails(ctx context.Context, mobileUserID string) (*models.User, error)
}
