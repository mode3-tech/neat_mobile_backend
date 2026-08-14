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

// OptimusValidator is implemented by the Optimus BaaS client. The Optimus API
// returns the same response envelope for successful and failed HTTP requests,
// so implementations must decode it before returning an error.
type OptimusValidator interface {
	ValidateBVN(ctx context.Context, request OptimusBVNValidationRequest) (*OptimusResponse, error)
	ValidateNIN(ctx context.Context, request OptimusNINValidationRequest) (*OptimusResponse, error)
}

// SessionIssuer is satisfied by *auth.Service - reused so registerv2 issues
// sessions the exact same way the old login/registration flow does.
type SessionIssuer interface {
	IssueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*auth.VerifiedDeviceResponse, error)
}
