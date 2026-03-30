package auth

type TokenType string

type AuthObject struct {
	AccessToken  string
	RefreshToken string
}

type LoginInitObject struct {
	Status       string
	Challenge    string
	SessionToken string
}

type WalletPayload struct {
	BVN         string
	FirstName   string
	LastName    string
	DateOfBirth string
}

const (
	LoginStatusChallengeRequired = "challenge_required"
	LoginStatusNewDeviceDetected = "new_device_detected"
)

type Provider string

const (
	ProviderTendar  Provider = "tendar"
	ProviderPrembly Provider = "prembly"
)
