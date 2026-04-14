package otp

type Purpose string
type Channel string
type Provider string

const (
	PurposeLogin          Purpose = "login"
	PurposeSignup         Purpose = "signup"
	PurposePasswordReset  Purpose = "password_reset"
	PurposePasswordChange Purpose = "password_change"
	PurposePinReset       Purpose = "pin_reset"
	PurposePinChange      Purpose = "pin_change"
)

const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
)

const (
	ProviderTermii Provider = "termii"
)
