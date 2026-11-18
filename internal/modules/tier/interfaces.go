package tier

import (
	"context"
	"encoding/json"
	"neat_mobile_app_backend/internal/ninfacevalidator"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/providers/nin"
)

type NINService interface {
	ValidateNIN(ctx context.Context, number string) (*nin.ValidationResponse, error)
}

type UserService interface {
	GetUserDetails(ctx context.Context, mobileUserID string) (*models.User, error)
	UpdateUserNIN(ctx context.Context, mobileUserID string, isNINVerified bool, ninHash string) error
}

type FaceService interface {
	ValidateNINWithFace(ctx context.Context, ninNumber, dateOfBirth, image string) (*ninfacevalidator.Result, error)
}

// WemaAccountResolver reports the user's Wema ALAT account number; ok is
// false when the user's wallet isn't a Wema one.
type WemaAccountResolver interface {
	WemaAccountNumber(ctx context.Context, mobileUserID string) (accountNumber string, ok bool, err error)
}

// WemaTierUpgrader upgrades a Wema ALAT account's tier with the bank.
type WemaTierUpgrader interface {
	UpgradeToTier2(ctx context.Context, accountNumber, nin, bvn, liveImageOfFace string) error
	UpgradeToTier3(ctx context.Context, accountNumber string, address ResidentialAddress) error
	// KYCStatus returns ALAT's raw KYC status data. Its shape is
	// undocumented, so it isn't decoded.
	KYCStatus(ctx context.Context, accountNumber string) (json.RawMessage, error)
}
