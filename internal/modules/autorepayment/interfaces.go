package autorepayment

import (
	"context"
	"neat_mobile_app_backend/internal/modules/wallet"
)

type ProvidusService interface {
	FetchBanks(ctx context.Context) ([]Bank, error)
	FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*BankDetails, error)
	GetCustomerDetails(ctx context.Context, customerID string) (*ProvidusCustomerDetailsResponse, error)
	InitiateTransfer(ctx context.Context, providusCustomerID string, transferInfo *wallet.TransferRequest) (*wallet.TransferResponse, error)
}
