package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess  = "access_token"
	TokenTypeRefresh = "refresh_token"
)

type Signer struct {
	secret string
}

func NewSigner(secret string) *Signer {
	return &Signer{secret: secret}
}

/*
 * @brief Generate a JWT token for the given user ID.
 * @param userID The user ID to generate the token for.
 * @return The generated JWT token.
 * @return An error if the token generation fails.
 */

func (s *Signer) Generate(userID, tokenType string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"typ": tokenType,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

func (s *Signer) CrossCheck(tokenString, expectedType string) (string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {

		//Check algorithm
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.secret), nil
	})

	if err != nil || token == nil || !token.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
	}

	typ, ok := claims["typ"].(string)
	if !ok || typ != expectedType {
		return "", errors.New("invalid token type")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("invalid subject")
	}

	return "", errors.New("invalid token")
}

func (s *Signer) ValidAccessToken(tokenString string) bool {
	_, err := s.CrossCheck(tokenString, TokenTypeAccess)
	return err == nil
}

func (s *Signer) ValidRefreshToken(tokenString string) bool {
	_, err := s.CrossCheck(tokenString, TokenTypeRefresh)
	return err == nil
}
