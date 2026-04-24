package loanproduct

import "context"

type CoreCustomerFinder interface {
	MatchCustomerByBVN(ctx context.Context, bvn string) (*CoreCustomerMatchData, error)
}

type CoreLoanFinder interface {
	GetCustomerLoans(ctx context.Context, customerID string) ([]CoreCustomerLoanItem, error)
	GetLoanDetail(ctx context.Context, loanID string) (*CoreLoanDetail, error)
	GetLoanRepayments(ctx context.Context, loanID string) (*[]LoanRepayment, error)
}

type ManualRepayer interface {
	MakeManualRepayment(ctx context.Context, req RepaymentRequest) (*ManualRepaymentResponse, error)
}

type RepaymentFundTransferrer interface {
	TransferForLoanRepayment(ctx context.Context, mobileUserID string, amountNaira int64) error
}
