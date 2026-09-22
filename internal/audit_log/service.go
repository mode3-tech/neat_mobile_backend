package auditlog

import "context"

type Service struct {
	Repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{Repo: repo}
}

func (s *Service) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	return s.Repo.CreateAuditLog(ctx, log)
}
