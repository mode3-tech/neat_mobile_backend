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

// AlatResidentialAddress is the customer's address for Tier 2 face account
// creation. ALAT requires the whole object; a field with no value is sent as
// an empty string.
type AlatResidentialAddress struct {
	BuildingNumber        string
	Apartment             string
	Street                string
	City                  string
	Town                  string
	State                 string
	Lga                   string
	Lcda                  string
	Landmark              string
	AdditionalInformation string
	Country               string
	FullAddress           string
	PostalCode            string
}

// AlatTier2FaceRequest holds parameters for Step 1c (Tier 2 Face Account Creation).
type AlatTier2FaceRequest struct {
	BVN                string
	NIN                string
	PhoneNumber        string
	EmailAddress       string
	ResidentialAddress *AlatResidentialAddress
	LiveImageOfFace    string
	CorrelationID      string
}

// AlatTier2Result is ALAT's answer to a tier 2 account creation request:
// the tracking ID (if it sent one) and its own message, e.g. "your account
// will be created shortly", which the client can show on a pending screen.
type AlatTier2Result struct {
	TrackingID string
	Message    string
}

// AlatValidator defines Wema ALAT account creation operations needed by registerv2.
// Defined with local types to avoid import cycle between registerv2 and baas.
// Registration uses the face biometric (liveness) API only; ALAT's BVN + OTP
// account creation API is not part of it.
type AlatValidator interface {
	GetPartnershipAccountDetails(ctx context.Context, phoneNumber string) (*AlatAccountDetails, error)
	PartnerDebitRestrictionManagement(ctx context.Context, pndType, accountNumber string) error
	PostTier2PartnershipAccountWithFace(ctx context.Context, req AlatTier2FaceRequest) (AlatTier2Result, error)
	GetAddressDropDownList(ctx context.Context) (any, error)
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
// sessions the exact same way the old login/registration flow does, and can
// run KYC provider validations (Tendar/Prembly).
type SessionIssuer interface {
	IssueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*auth.VerifiedDeviceResponse, error)
	ValidateBVN(ctx context.Context, bvn string) (*auth.BVNInfo, error)
	ValidateBVNWithFace(ctx context.Context, payload auth.BVNWithFaceValidationRequest) (*auth.BVNWithFaceInfo, error)
}
