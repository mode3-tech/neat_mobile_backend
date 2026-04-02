package transaction

import "context"

type Service struct {
	repo *Repository
}

func NewServie(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) FetchTransactions(ctx context.Context) {}
