package appversion

import (
	"context"
	"errors"

	"gorm.io/gorm"

	appErr "neat_mobile_app_backend/internal/errors"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetAppVersion(ctx context.Context, os string) (*AppVersionInfo, error) {
	if os == "" {
		os = "android"
	}
	appVersionInfo, err := s.repo.GetAppVersionInfo(ctx, os)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound){
			return nil, appErr.ErrAppOSNotFound
		}
		return nil, err
	}
	return appVersionInfo, nil
}
