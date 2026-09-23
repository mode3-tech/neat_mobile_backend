// Package ninfacevalidator is a thin, injectable wrapper around NIN-with-face
// validation, supporting both Prembly and Tendar. It has no dependency on
// VerificationRecord/FaceCheckRecord - callers already have the NIN/DOB they
// need (e.g. tier.Service reads them off wallet_users) and are responsible
// for persisting whatever they do with the result.
package ninfacevalidator

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/providers/nin"

	"gorm.io/gorm"
)

type Provider string

const (
	ProviderPrembly Provider = "prembly"
	ProviderTendar  Provider = "tendar"
)

// validationProviderPreferenceKey mirrors ninvalidator's - the same
// system_preferences row drives provider selection everywhere.
const validationProviderPreferenceKey = "bvn_validation_provider"

// PremblyClient and TendarClient are separate interfaces, not one shared
// Client, because the two providers' real methods aren't structurally
// identical - different parameter counts/order and different response types.
type PremblyClient interface {
	ValidateNINWithFace(ctx context.Context, image, number, dateOfBirth string) (*nin.PremblyNINWithFaceValidationSuccessResponse, error)
}

type TendarClient interface {
	ValidateNINWithFace(ctx context.Context, number, imageURL string) (*nin.NINWithFaceResponse, error)
}

// Result is the provider-neutral outcome both providers get normalized into.
type Result struct {
	Matched      bool
	Confidence   float64
	Message      string
	ResponseCode string
}

type Validator struct {
	db      *gorm.DB
	prembly PremblyClient
	tendar  TendarClient
}

func New(db *gorm.DB, prembly PremblyClient, tendar TendarClient) *Validator {
	return &Validator{db: db, prembly: prembly, tendar: tendar}
}

// ValidateNINWithFace resolves the configured provider and runs a live face
// match for ninNumber/dateOfBirth against image. Callers own persisting the
// outcome.
func (v *Validator) ValidateNINWithFace(ctx context.Context, ninNumber, dateOfBirth, image string) (*Result, error) {
	ninNumber = strings.TrimSpace(ninNumber)
	dateOfBirth = strings.TrimSpace(dateOfBirth)
	image = strings.TrimSpace(image)

	if v.resolveProvider(ctx) == ProviderTendar {
		resp, err := v.tendar.ValidateNINWithFace(ctx, ninNumber, image)
		if err != nil {
			log.Printf("ninfacevalidator: tendar provider call failed: %v", err)
			return nil, translateProviderError(err, appErr.ErrValidatingNINWithFace)
		}
		return &Result{
			Matched:      resp.FaceData.Matched,
			Confidence:   resp.FaceData.Confidence,
			Message:      resp.FaceData.Message,
			ResponseCode: resp.FaceData.ResponseCode,
		}, nil
	}

	resp, err := v.prembly.ValidateNINWithFace(ctx, image, ninNumber, dateOfBirth)
	if err != nil {
		log.Printf("ninfacevalidator: prembly provider call failed: %v", err)
		return nil, translateProviderError(err, appErr.ErrValidatingNINWithFace)
	}
	return &Result{
		Matched:      resp.FaceData.Status,
		Confidence:   resp.FaceData.Confidence,
		Message:      resp.FaceData.Message,
		ResponseCode: resp.FaceData.ResponseCode,
	}, nil
}

// resolveProvider mirrors ninvalidator.Validator.resolveProvider exactly -
// duplicated rather than imported so this package doesn't depend on
// ninvalidator (kept independent, same rationale as translateProviderError
// below).
func (v *Validator) resolveProvider(ctx context.Context) Provider {
	var pref models.SystemPreference
	if err := v.db.WithContext(ctx).
		Where("preference_key = ?", validationProviderPreferenceKey).
		First(&pref).Error; err != nil {
		log.Printf("ninfacevalidator: failed to resolve validation provider from system_preferences; forcing tendar: %v", err)
		return ProviderTendar
	}
	switch Provider(strings.ToLower(strings.TrimSpace(pref.PreferenceValue))) {
	case ProviderPrembly:
		return ProviderPrembly
	default:
		return ProviderTendar
	}
}

// providerInfraCodes/translateProviderError/classifyProviderFailure mirror
// auth.translateProviderError - duplicated rather than imported to keep this
// package free of any internal/modules dependency (avoids a cycle back
// through auth). Now handles both PremblyError and TendarError, since both
// providers are live here.
var providerInfraCodes = map[string]struct{}{
	"CLIENT_ERROR":     {},
	"TIMEOUT":          {},
	"NETWORK_ERROR":    {},
	"INVALID_RESPONSE": {},
}

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
