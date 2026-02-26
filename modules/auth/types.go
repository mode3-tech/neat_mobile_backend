package auth

type TokenType string

type AuthObject struct {
	AccessToken  string
	RefreshToken string
}
