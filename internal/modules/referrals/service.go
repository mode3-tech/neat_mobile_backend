package referrals

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GenerateReferralCode(ctx context.Context, mobileUserID string) (string, error) {
	return "", nil
}
