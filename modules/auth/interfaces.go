package auth

import (
	"context"
	"neat_mobile_app_backend/internal"
	"neat_mobile_app_backend/modules/loanproduct"
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
}

type TendarValidation interface {
	ValidateBVNWithTendar(ctx context.Context, BVN string) (*bvn.TendarBVNValidationSuccessResponse, error)
}

type PremblyValidation interface {
	ValidateBVNWithPrembly(ctx context.Context, BVN string) (*bvn.PremblyBVNValidationSuccessResponse, error)
}

type BVNProviderSource interface {
	GetCurrentProvider(ctx context.Context) (Provider, error)
}

type NINValidation interface {
	ValidateNIN(ctx context.Context, nin string) (*nin.PremblyNINValidationSuccessResponse, error)
}

type CoreCustomerFinder interface {
	MatchCustomerByBVN(ctx context.Context, bvn string) (*loanproduct.CoreCustomerMatchData, error)
}

type CBACustomerUpdater interface {
	UpdateCBACustomerBankInfo(ctx context.Context, coreCustomerID string, customerUpdate *internal.CustomerUpdateRequest) (*internal.CustomerUpdateResponse, error)
}
