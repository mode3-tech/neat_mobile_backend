package loanproduct

import "context"

type CoreCustomerFinder interface {
	MatchCustomerByBVN(ctx context.Context, bvn string) (*CoreCustomerMatchData, error)
}

type CoreLoanFinder interface {
	GetCustomerLoans(ctx context.Context, customerID string) ([]CoreCustomerLoanItem, error)
	GetLoanDetail(ctx context.Context, loanID string) (*CoreLoanDetail, error)
}
