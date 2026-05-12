package auth

import (
	"context"
	"neat_mobile_app_backend/models"
	"strings"

	"gorm.io/gorm"
)

type DBProviderSource struct {
	db *gorm.DB
}

func NewDBProviderSource(db *gorm.DB) *DBProviderSource {
	return &DBProviderSource{db: db}
}

func (s *DBProviderSource) GetCurrentProvider(ctx context.Context) (Provider, error) {
	var pref models.SystemPreference
	if err := s.db.WithContext(ctx).
		Where("preference_key = ?", "bvn_validation_provider").
		First(&pref).Error; err != nil {
		return ProviderPrembly, err
	}
	switch Provider(strings.ToLower(strings.TrimSpace(pref.PreferenceValue))) {
	case ProviderTendar:
		return ProviderTendar, nil
	default:
		return ProviderPrembly, nil
	}
}
