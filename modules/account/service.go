package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type Service struct {
	repo         *Repository
	loanProvider LoanProvider
}

func NewService(repo *Repository, loanProvider LoanProvider) *Service {
	return &Service{repo: repo, loanProvider: loanProvider}
}

func (s *Service) GetAccountSummary(ctx context.Context, mobileUserID, deviceID string) (*AccountSummary, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	deviceID = strings.TrimSpace(deviceID)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if deviceID == "" {
		return nil, errors.New("device ID is required")
	}

	if _, err := s.repo.GetDevice(ctx, mobileUserID, deviceID); err != nil {
		return nil, fmt.Errorf("failed to verify device: %w", err)
	}

	accountInfo, err := s.repo.GetAccountSummary(ctx, mobileUserID)
	if err != nil {
		return nil, err
	}

	var loanBalance float64
	var activeLoans []ActiveLoan

	loans, err := s.loanProvider.GetAllLoans(ctx, mobileUserID)
	if err == nil {
		for _, loan := range loans {
			loanBalance += loan.OutstandingBalance
			if strings.ToLower(loan.Status) != "active" {
				continue
			}
			activeLoans = append(activeLoans, ActiveLoan{
				LoanID:           loan.LoanID,
				LoanNumber:       loan.LoanNumber,
				LoanAmount:       loan.PrincipalAmount,
				TotalRepayment:   loan.OutstandingBalance,
				MonthlyRepayment: loan.NextDueAmount,
				NextDueDate:      loan.NextDueDate,
			})
		}
	}

	if activeLoans == nil {
		activeLoans = []ActiveLoan{}
	}

	return &AccountSummary{
		FullName:         strings.TrimSpace(accountInfo.FirstName + " " + accountInfo.LastName),
		BankName:         accountInfo.BankName,
		AccountNumber:    accountInfo.AccountNumber,
		AvailableBalance: accountInfo.AvailableBalance,
		WalletID:         accountInfo.InternalWalletID,
		LoanBalance:      loanBalance,
		ActiveLoans:      activeLoans,
	}, nil
}

func AccountStatementRequest(ctx context.Context, mobileUserID string) {

}

func (s *Service) UpdateProfile(ctx context.Context, mobileUserID, deviceID string, req UpdateProfileRequest) error {
	if strings.TrimSpace(mobileUserID) == "" {
		return errors.New("user id is missing")
	}

	if err := s.repo.UpdateProfile(ctx, mobileUserID, req); err != nil {
		return err
	}

	return nil
}
