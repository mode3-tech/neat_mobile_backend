package loanproduct

import (
	"context"
	"neat_mobile_app_backend/modules/device"
)

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

type DeviceVerifier interface {
	VerifyUserDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error)
}
