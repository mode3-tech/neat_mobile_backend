package referrals

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RedeemReferralCode(ctx context.Context, mobileUserID, code string) error {
	referral, err := s.repo.FindReferralByCode(ctx, code)
	if err != nil {
		return err
	}
	if referral == nil {
		return nil
	}

	redeemedReferral := &ReferralRedemption{
		ID:             uuid.NewString(),
		ReferrerUserID: referral.MobileUserID,
		ReferredUserID: mobileUserID,
	}

	if err := s.repo.RedeemReferral(ctx, redeemedReferral); err != nil {
		return err
	}

	return nil
}
