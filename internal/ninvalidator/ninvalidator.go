// Package ninvalidator resolves which identity provider (Prembly or Tendar)
// should handle NIN validation, using the same system_preferences toggle
// auth.DBProviderSource already reads, and exposes one injectable entrypoint
// so callers don't have to duplicate that provider-resolution logic
// themselves - just inject a *Validator wherever a NINService/NINValidation
// shaped dependency is expected.
package ninvalidator

import (
	"context"
	"log"
	"strings"

	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/providers/nin"

	"gorm.io/gorm"
)

type Provider string

const (
	ProviderPrembly Provider = "prembly"
	ProviderTendar  Provider = "tendar"
)

// validationProviderPreferenceKey mirrors auth.validationProviderPreferenceKey.
// The same system_preferences row drives BVN and NIN provider selection
// across the app - the key name is historical, it predates NIN routing.
const validationProviderPreferenceKey = "bvn_validation_provider"

// Client is satisfied by both the Prembly and Tendar NIN provider clients.
type Client interface {
	ValidateNIN(ctx context.Context, number string) (*nin.ValidationResponse, error)
}

// Validator is the shared, injectable NIN validation entrypoint. It
// implements the same single-method shape (ValidateNIN(ctx, number) (*nin.ValidationResponse, error))
// already used as a dependency interface elsewhere (e.g. tier.NINService),
// so it can be injected directly without callers needing an adapter.
type Validator struct {
	db      *gorm.DB
	prembly Client
	tendar  Client
}

func New(db *gorm.DB, prembly, tendar Client) *Validator {
	return &Validator{db: db, prembly: prembly, tendar: tendar}
}

// ValidateNIN resolves the currently-configured provider and validates
// number against it.
func (v *Validator) ValidateNIN(ctx context.Context, number string) (*nin.ValidationResponse, error) {
	_, client := v.providerFor(ctx)
	return client.ValidateNIN(ctx, number)
}

func (v *Validator) providerFor(ctx context.Context) (Provider, Client) {
	if v.resolveProvider(ctx) == ProviderPrembly {
		return ProviderPrembly, v.prembly
	}
	return ProviderTendar, v.tendar
}

// resolveProvider reads the single system_preferences row that governs BVN
// and NIN validation. Any failure - no row, DB error - falls back to Tendar,
// matching auth.DBProviderSource's fallback behavior.
func (v *Validator) resolveProvider(ctx context.Context) Provider {
	var pref models.SystemPreference
	if err := v.db.WithContext(ctx).
		Where("preference_key = ?", validationProviderPreferenceKey).
		First(&pref).Error; err != nil {
		log.Printf("ninvalidator: failed to resolve validation provider from system_preferences; forcing tendar: %v", err)
		return ProviderTendar
	}
	switch Provider(strings.ToLower(strings.TrimSpace(pref.PreferenceValue))) {
	case ProviderPrembly:
		return ProviderPrembly
	default:
		return ProviderTendar
	}
}
