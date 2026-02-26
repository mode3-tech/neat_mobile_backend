package jwt

import (
	"errors"
	"neat_mobile_app_backend/modules/auth"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess  auth.TokenType = "access_token"
	TokenTypeRefresh auth.TokenType = "refresh_token"
)

type Signer struct {
	secret string
}

func NewSigner(secret string) *Signer {
	return &Signer{secret: secret}
}

// IssueAccessToken takes userID and sessionID and returns access token or error if any
func (s *Signer) IssueAccessToken(userID, sid string) (string, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)
	claims := jwt.MapClaims{
		"sub": userID,
		"sid": sid,
		"typ": TokenTypeAccess,
		"iat": now.Unix(),
		"exp": expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

// IssueRefreshToken takes userID, sessionID and jti and returns refresh token or error if any
func (s *Signer) IssueRefreshToken(userID, sid string) (string, string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	jti := uuid.New().String()

	claims := jwt.MapClaims{
		"sub": userID,
		"sid": sid,
		"jti": jti,
		"typ": TokenTypeRefresh,
		"iat": now.Unix(),
		"exp": expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", "", time.Time{}, err
	}

	return signedToken, jti, expiresAt, nil
}

// ParseAndValidate takes tokenString and expectedType and returns claims or error if any
func (s *Signer) ParseAndValidate(tokenString string, expectedType auth.TokenType) (jwt.MapClaims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)

	token, err := parser.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.secret), nil
	})

	if err != nil || token == nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}

	typ, ok := claims["typ"].(string)
	if !ok || typ != string(expectedType) {
		return nil, errors.New("invalid token type")
	}

	return claims, nil
}

func (s *Signer) CrossCheck(tokenString string, expectedTokenType auth.TokenType) (string, error) {
	claims, err := s.ParseAndValidate(tokenString, expectedTokenType)
	if err != nil {
		return "", err
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return "", errors.New("invalid subject")
	}

	return sub, nil
}

func (s *Signer) ValidAccessToken(tokenString string) bool {
	_, err := s.ParseAndValidate(tokenString, TokenTypeAccess)
	return err == nil
}

func (s *Signer) ValidRefreshToken(tokenString string) bool {
	_, err := s.ParseAndValidate(tokenString, TokenTypeRefresh)
	return err == nil
}

// ExtractRefreshTokenIdentifiers takes refresh token string and returns subject, sessionID, tokenID or error if any
func (s *Signer) ExtractRefreshTokenIdentifiers(tokenString string) (sub, sid, jti string, err error) {
	claims, err := s.ParseAndValidate(tokenString, TokenTypeRefresh)
	if err != nil {
		return "", "", "", err
	}

	sub, ok := claims["sub"].(string)

	if !ok || sub == "" {
		return "", "", "", errors.New("invalid subject")
	}

	sid, ok = claims["sid"].(string)
	if !ok || sid == "" {
		return "", "", "", errors.New("invalid session id")
	}

	jti, ok = claims["jti"].(string)
	if !ok || jti == "" {
		return "", "", "", errors.New("invalid token id")
	}

	return sub, sid, jti, nil
}

func (s *Signer) ExtractAccessTokenIdentifiers(tokenString string) (sub string, sid string, err error) {
	claims, err := s.ParseAndValidate(tokenString, TokenTypeAccess)
	if err != nil {
		return "", "", err
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", "", errors.New("invalid subject")
	}

	sid, ok = claims["sid"].(string)
	if !ok || sid == "" {
		return "", "", errors.New("invalid session id")
	}

	return sub, sid, nil
}
