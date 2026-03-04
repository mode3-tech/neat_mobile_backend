package auth

type TokenType string

type AuthObject struct {
	AccessToken  string
	RefreshToken string
}

type Provider string

const (
	ProviderTendar  Provider = "tendar"
	ProviderPrembly Provider = "prembly"
)
