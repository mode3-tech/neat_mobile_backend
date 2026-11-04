package registerv2

import (
	"context"
	"neat_mobile_app_backend/internal/modules/auth"
)

// ProviderPreferenceRepository is the narrow lookup Service needs to decide
// which BaaS provider to use for a new registration. Satisfied by *Repository,
// kept as an interface so it can be swapped/mocked independently of the rest
// of the registration data access.
type ProviderPreferenceRepository interface {
	GetProviderPreference(ctx context.Context) (*ProviderPreference, error)
}

// AlatAccountDetails holds account info retrieved from Wema ALAT.
type AlatAccountDetails struct {
	AccountNumber string
	FirstName     string
	LastName      string
	Email         string
	PhoneNumber   string
}

// AlatValidator defines Wema ALAT account creation operations needed by registerv2.
// Defined with local types to avoid import cycle between registerv2 and baas.
type AlatValidator interface {
	PostPartnershipAccountCreationWithBvn(ctx context.Context, phone, email, bvn string) (trackingID string, err error)
	ValidateBVNandEnqueueAccountCreation(ctx context.Context, phone, otp, trackingID string) error
	GetPartnershipAccountDetails(ctx context.Context, phoneNumber string) (*AlatAccountDetails, error)
	PartnerDebitRestrictionManagement(ctx context.Context, pndType, accountNumber string) error
	ResendOtp(ctx context.Context, trackingID string) error
}

// OptimusValidator is implemented by the Optimus BaaS client. The Optimus API
// returns the same response envelope for successful and failed HTTP requests,
// so implementations must decode it before returning an error.
type OptimusValidator interface {
	ValidateBVN(ctx context.Context, request OptimusBVNValidationRequest) (*OptimusResponse, error)
	ValidateNIN(ctx context.Context, request OptimusNINValidationRequest) (*OptimusResponse, error)
	// VerifyOTPWithOptimus and ResendOTPWithOptimus are generic: the reference id
	// alone identifies which OTP challenge to act on, regardless of whether it
	// came from BVN validation, NIN validation, or anything else.
	VerifyOTPWithOptimus(ctx context.Context, phone, otpToken, email, referenceID string) error
	// ResendOTPWithOptimus returns the new reference id Optimus issues for the
	// resent OTP - callers must use it for the next verify/resend, not the old one.
	ResendOTPWithOptimus(ctx context.Context, referenceID string) (string, error)
}

// SessionIssuer is satisfied by *auth.Service - reused so registerv2 issues
// sessions the exact same way the old login/registration flow does.
type SessionIssuer interface {
	IssueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*auth.VerifiedDeviceResponse, error)
}
