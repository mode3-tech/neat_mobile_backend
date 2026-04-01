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

	user, err := s.repo.GetUser(ctx, mobileUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	customerWallet, err := s.repo.GetCustomerWallet(ctx, mobileUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch wallet: %w", err)
	}

	var loanBalance float64
	var activeLoans []ActiveLoan

	loans, err := s.loanProvider.GetAllLoans(ctx, mobileUserID)
	if err == nil {
		for _, loan := range loans {
			loanBalance += loan.OutstandingBalance
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
		FullName:         strings.TrimSpace(user.FirstName + " " + user.LastName),
		AccountNumber:    customerWallet.AccountNumber,
		AvailableBalance: customerWallet.AvailableBalance,
		LoanBalance:      loanBalance,
		ActiveLoans:      activeLoans,
	}, nil
}
