package sms

import (
	"context"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateOutgoingSMS(ctx context.Context, id, phone, message, recipient string) (*OutgoingSMS, error) {
	return s.repo.CreateOutgoingSMS(ctx, id, phone, message, recipient)
}

func (s *Service) UpdateOutgoingSMS(ctx context.Context, id string, status OutgoingSMSStatus, sentAt *time.Time, reasonForFailure string) error {
	return s.repo.UpdateOutgoingSMSStatus(ctx, id, status, sentAt, reasonForFailure)
}

func (s *Service) GetPendingOutgoingSMS(ctx context.Context, retryBackoff time.Duration) ([]OutgoingSMS, error) {
	return s.repo.FetchPendingOutgoingSMS(ctx, retryBackoff)
}
