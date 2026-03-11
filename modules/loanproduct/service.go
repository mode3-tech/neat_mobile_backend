package loanproduct

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetAllLoanProducts(ctx context.Context) ([]LoanProduct, error) {
	return s.repo.GetAllLoanProducts(ctx)
}
