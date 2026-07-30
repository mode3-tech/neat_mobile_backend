package auth

import (
	"errors"
	"log"
	"net/http"

	appErr "neat_mobile_app_backend/internal/errors"
)

// providerInfraCodes are locally-synthesized classifications that describe a failure
// of the integration itself — a missing API key, a transport problem, an
// undecodable body. None are actionable by the end user and none say anything
// about the identity being looked up, so they must not be reported as "not found".
var providerInfraCodes = map[string]struct{}{
	"CLIENT_ERROR":     {},
	"TIMEOUT":          {},
	"NETWORK_ERROR":    {},
	"INVALID_RESPONSE": {},
}

// translateProviderError converts a typed identity-provider error (*PremblyError or
// *TendarError) into a domain error the response mapper already understands.
//
// notFound is the subject-specific error used when the provider says the identity
// does not exist — appErr.ErrNINNotFound or appErr.ErrBVNNotFound depending on the
// caller. Errors that are not typed provider errors are returned unchanged.
func translateProviderError(err error, notFound error) error {
	if err == nil {
		return nil
	}

	var premblyErr *appErr.PremblyError
	if errors.As(err, &premblyErr) {
		return classifyProviderFailure("prembly", premblyErr.Status, premblyErr.Code, premblyErr.Retryable, notFound, err)
	}

	var tendarErr *appErr.TendarError
	if errors.As(err, &tendarErr) {
		return classifyProviderFailure("tendar", tendarErr.Status, tendarErr.Code, tendarErr.Retryable, notFound, err)
	}

	return err
}

func classifyProviderFailure(provider string, status int, code string, retryable bool, notFound, original error) error {
	if _, infra := providerInfraCodes[code]; infra || retryable {
		// Log loudly: an unconfigured key or a provider outage is ours to fix, and
		// the generic 503 the user sees carries none of this detail.
		log.Printf("identity provider %s unavailable: status=%d code=%s err=%v", provider, status, code, original)
		return appErr.ErrProviderServiceUnavailable
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		log.Printf("identity provider %s rejected our credentials: status=%d code=%s", provider, status, code)
		return appErr.ErrProviderServiceUnavailable
	case status == http.StatusNotFound || status == http.StatusUnprocessableEntity || code == "VALIDATION_ERROR":
		return notFound
	default:
		return original
	}
}
