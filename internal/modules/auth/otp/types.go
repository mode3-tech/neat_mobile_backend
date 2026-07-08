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
	// PurposeSubmittedContact verifies an additional phone/email the user
	// submits during signup (e.g. an alternate reachable phone when the BVN
	// phone is unreachable). Unlike PurposeSignup, its verified record keeps
	// the channel-based type (phone/email) rather than the umbrella "otp" type.
	PurposeSubmittedContact Purpose = "submitted_contact"
)

const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
	ChannelNone  Channel = ""
)

const (
	ProviderTermii Provider = "termii"
)
