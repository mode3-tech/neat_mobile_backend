package auth

import (
	"context"
	"neat_mobile_app_backend/models"
	"strings"

	"gorm.io/gorm"
)

// validationProviderPreferenceKey is the system_preferences row that selects the
// identity provider for BOTH BVN and NIN lookups. The key name is historical — it
// predates NIN routing. The CBA admin service owns writes to this row; this service
// only reads it.
const validationProviderPreferenceKey = "bvn_validation_provider"

type DBProviderSource struct {
	db *gorm.DB
}

func NewDBProviderSource(db *gorm.DB) *DBProviderSource {
	return &DBProviderSource{db: db}
}

func (s *DBProviderSource) GetCurrentProvider(ctx context.Context) (Provider, error) {
	var pref models.SystemPreference
	if err := s.db.WithContext(ctx).
		Where("preference_key = ?", validationProviderPreferenceKey).
		First(&pref).Error; err != nil {
		return ProviderTendar, err
	}
	// Only tendar and prembly are valid identity providers. Anything else — a blank
	// value, a typo, or ProviderOptimus (which is a wallet provider, not a KYC one) —
	// resolves to Tendar, matching the fallback used everywhere else.
	switch Provider(strings.ToLower(strings.TrimSpace(pref.PreferenceValue))) {
	case ProviderPrembly:
		return ProviderPrembly, nil
	default:
		return ProviderTendar, nil
	}
}
