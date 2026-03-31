package wallet

import "context"

type Service struct {
	repo            *Repository
	providusService ProvidusService
}

func NewService(repo *Repository, providusService ProvidusService) *Service {
	return &Service{repo: repo, providusService: providusService}
}

func (s *Service) FetchBanks(ctx context.Context) ([]Bank, error) {
	return s.providusService.FetchBanks(ctx)
}

func (s *Service) FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*BankDetails, error) {
	return s.providusService.FetchBankDetails(ctx, accountNumber, bankCode)
}
