package otp

type Purpose string
type Channel string
type Provider string

const (
	PurposeLogin         Purpose = "login"
	PurposeSignup        Purpose = "signup"
	PurposePasswordReset Purpose = "password_reset"
)

const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
)

const (
	ProviderTermii Provider = "termii"
)
