package account

import (
	"context"
	"neat_mobile_app_backend/modules/loanproduct"
)

type LoanProvider interface {
	GetAllLoans(ctx context.Context, userID string) ([]loanproduct.CoreCustomerLoanItem, error)
}
