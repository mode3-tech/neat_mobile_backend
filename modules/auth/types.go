package auth

type TokenType string

type AuthObject struct {
	AccessToken  string
	RefreshToken string
}

type BVNServiceType string

const (
	TendarServiceType  BVNServiceType = "tendar"
	PremblyServiceType BVNServiceType = "prembly"
)
