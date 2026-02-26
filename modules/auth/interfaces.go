package auth

import (
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
