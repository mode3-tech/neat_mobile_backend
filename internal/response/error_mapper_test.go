package response

import (
	"fmt"
	appErr "neat_mobile_app_backend/internal/errors"
	"net/http"
	"testing"
)

// These guard the safety net: before it existed, every typed identity-provider
// failure fell through to a bare 500 INTERNAL_SERVER_ERROR.
func TestMapError_IdentityProviderErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:        "prembly unconfigured key",
			err:         &appErr.PremblyError{Status: 500, Code: "CLIENT_ERROR", Message: "not configured"},
			wantStatus:  http.StatusServiceUnavailable,
			wantCode:    "SERVICE_UNAVAILABLE",
			wantMessage: appErr.ErrProviderServiceUnavailable.Error(),
		},
		{
			name:        "tendar timeout",
			err:         &appErr.TendarError{Status: 408, Code: "TIMEOUT", Retryable: true},
			wantStatus:  http.StatusServiceUnavailable,
			wantCode:    "SERVICE_UNAVAILABLE",
			wantMessage: appErr.ErrProviderServiceUnavailable.Error(),
		},
		{
			name:        "prembly rate limit",
			err:         &appErr.PremblyError{Status: 429, Code: "HTTP_ERROR", Retryable: true},
			wantStatus:  http.StatusServiceUnavailable,
			wantCode:    "SERVICE_UNAVAILABLE",
			wantMessage: appErr.ErrProviderServiceUnavailable.Error(),
		},
		{
			name:        "bad api key is never surfaced to the client",
			err:         &appErr.PremblyError{Status: 401, Code: "HTTP_ERROR", Message: "invalid api key supplied"},
			wantStatus:  http.StatusServiceUnavailable,
			wantCode:    "SERVICE_UNAVAILABLE",
			wantMessage: appErr.ErrProviderServiceUnavailable.Error(),
		},
		{
			name:        "provider rejection forwards its message",
			err:         &appErr.TendarError{Status: 422, Code: "VALIDATION_ERROR", Message: "record not found"},
			wantStatus:  http.StatusUnprocessableEntity,
			wantCode:    "IDENTITY_PROVIDER_ERROR",
			wantMessage: "record not found",
		},
		{
			name:        "blank provider message gets a usable fallback",
			err:         &appErr.PremblyError{Status: 400, Code: "HTTP_ERROR"},
			wantStatus:  http.StatusUnprocessableEntity,
			wantCode:    "IDENTITY_PROVIDER_ERROR",
			wantMessage: "Identity verification could not be completed, please try again",
		},
		{
			name:        "wrapped provider errors are still recognised",
			err:         fmt.Errorf("validate nin: %w", &appErr.TendarError{Status: 502, Code: "NETWORK_ERROR", Retryable: true}),
			wantStatus:  http.StatusServiceUnavailable,
			wantCode:    "SERVICE_UNAVAILABLE",
			wantMessage: appErr.ErrProviderServiceUnavailable.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MapError(tc.err)
			if got.Status != tc.wantStatus {
				t.Errorf("status = %d, want %d", got.Status, tc.wantStatus)
			}
			if got.Error.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", got.Error.Code, tc.wantCode)
			}
			if got.Error.Message != tc.wantMessage {
				t.Errorf("message = %q, want %q", got.Error.Message, tc.wantMessage)
			}
		})
	}
}
