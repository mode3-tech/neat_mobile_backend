package user

import (
	"context"
	"neat_mobile_app_backend/models"
	"time"
)

type Service struct {
	Repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{Repo: repo}
}

func (s *Service) SetUserAsClosed(ctx context.Context, mobileUserID string, closedAt time.Time) error {
	return s.Repo.SetUserAsClosed(ctx, mobileUserID, closedAt)
}

func (s *Service) GetUserDetails(ctx context.Context, mobileUserID string) (*models.User, error) {
	return s.Repo.GetUserDetails(ctx, mobileUserID)
}
