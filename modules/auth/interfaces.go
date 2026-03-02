package auth

import (
	"context"
	"neat_mobile_app_backend/providers/bvn"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTSigner interface {
	IssueAccessToken(userID, sid string) (string, error)
	IssueRefreshToken(userID, sid string) (string, string, time.Time, error)
	ParseAndValidate(tokenString string, expectedType TokenType) (jwt.MapClaims, error)
	CrossCheck(tokenString string, expectedTokenType TokenType) (string, error)
	ValidAccessToken(tokenString string) bool
	ValidRefreshToken(tokenString string) bool
	ExtractAccessTokenIdentifiers(tokenString string) (string, string, error)
	ExtractRefreshTokenIdentifiers(tokenString string) (string, string, string, error)
}

type TendarValidation interface {
	ValidateBVNWithTendar(ctx context.Context, BVN string) (*bvn.TendarBVNValidationSuccessResponse, error)
}

type PremblyValidation interface {
	ValidateBVNWithPrembly(ctx context.Context, BVN string) (*bvn.PremblyBVNValidationSuccessResponse, error)
}
