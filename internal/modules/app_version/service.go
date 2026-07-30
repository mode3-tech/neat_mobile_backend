package appversion

import "context"

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
		return nil, err
	}
	return appVersionInfo, nil
}
