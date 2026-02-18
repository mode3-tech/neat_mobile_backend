package auth

type JWTSigner interface {
	Generate(userID, tokenType string) (string, error)
	CrossCheck(tokenString, tokenType string) (string, error)
	ValidAccessToken(tokenString string) bool
	ValidRefreshToken(tokenString string) bool
}
