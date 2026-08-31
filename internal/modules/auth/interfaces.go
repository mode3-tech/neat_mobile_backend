package auth

import (
	"context"
	"neat_mobile_app_backend/internal"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/modules/loanproduct"
	"neat_mobile_app_backend/providers/bvn"
	"neat_mobile_app_backend/providers/nin"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTSigner interface {
	// IssueAccessToken takes userID and sessionID and returns access token or error if any
	IssueAccessToken(userID, sid string) (string, error)
	IssueRefreshToken(userID, sid string) (string, string, time.Time, error)
	ParseAndValidate(tokenString string, expectedType TokenType) (jwt.MapClaims, error)
	CrossCheck(tokenString string, expectedTokenType TokenType) (string, error)
	ValidAccessToken(tokenString string) bool
	ValidRefreshToken(tokenString string) bool
	ExtractAccessTokenIdentifiers(tokenString string) (string, string, error)
	ExtractRefreshTokenIdentifiers(tokenString string) (string, string, string, error)
}

type WalletService interface {
	GenerateWallet(ctx context.Context, walletInfo *WalletPayload) (*WalletResponse, error)
	LookupWalletByCustomerID(ctx context.Context, customerID string) (*WalletResponse, bool, error)
	LookupCustomerByPhone(ctx context.Context, phoneNumber string) (string, bool, error)
}

type TendarValidation interface {
	ValidateBVNWithTendar(ctx context.Context, BVN string) (*bvn.TendarBVNValidationSuccessResponse, error)
}

type PremblyValidation interface {
	ValidateBVNWithPrembly(ctx context.Context, BVN string) (*bvn.PremblyBVNValidationSuccessResponse, error)
	ValidateBVNWithFace(ctx context.Context, number, image string) (*bvn.PremblyBVNWithFaceResponse, error)
}

// ValidationProviderSource resolves which identity provider serves BVN *and* NIN
// lookups. A single preference governs both.
type ValidationProviderSource interface {
	GetCurrentProvider(ctx context.Context) (Provider, error)
}

// NINValidation is the provider-neutral NIN lookup, implemented by both
// providers/nin/prembly and providers/nin/tendar.
type NINValidation interface {
	ValidateNIN(ctx context.Context, number string) (*nin.ValidationResponse, error)
}

// NINFaceValidation is Prembly-only; Tendar exposes no NIN-with-face endpoint.
type NINFaceValidation interface {
	ValidateNINWithFace(ctx context.Context, image, numberNin, dateOfBirth string) (*nin.PremblyNINWithFaceValidationSuccessResponse, error)
}

type CoreCustomerFinder interface {
	MatchCustomerByBVN(ctx context.Context, bvn string) (*loanproduct.CoreCustomerMatchData, error)
}

type CBACustomerUpdater interface {
	UpdateCBACustomerBankInfo(ctx context.Context, coreCustomerID string, customerUpdate *internal.CustomerUpdateRequest) (*internal.CustomerUpdateResponse, error)
}

type DeviceVerifier interface {
	VerifyUserDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error)
}

type OptimusKYCValidation interface {
	VerifyOTPWithOptimus(ctx context.Context, phone, otpToken, email, referenceID string) error
}
