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

func (s *Service) GetAuditLogs(ctx context.Context, filters map[string]interface{}, page, limit int) ([]AuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	offset := (page - 1) * limit
	logs, total, err := s.Repo.GetAuditLogs(ctx, filters, limit, offset)
	if err != nil {
		return nil, total, err
	}
	return logs, total, nil
}
