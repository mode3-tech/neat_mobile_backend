package vas

import (
	"context"
)

type VASService interface {
	GetAirtime(ctx context.Context, requestID, uniqueCode, phoneNumber string, amount int64) (*ISPResponse, error)
	GetData(ctx context.Context, requestId, uniqueCode, phoneNumber string, amount int64) (*ISPResponse, error)
	ValidateElectricity(ctx context.Context, requestId, uniqueCode, accountNumber string, accountType AccountType) (*ElectricityValidationResponse, error)
	PayElectricityBill(ctx context.Context, requestId, uniqueCode, accountNumber, name, address, phoneNumber string, accountType AccountType, amount int64) (*PayElectricityResponse, error)
	ValidateCable(ctx context.Context, requestId, uniqueCode, accountNumber string, noOfMonth int) (*CableValidationResponse, error)
	PayCableBill(ctx context.Context, requestId, uniqueCode, accountNumber, accountType, name, phoneNumber string, noOfMonth int, amount int64) (*PayCableResponse, error)
}

type WalletService interface {
	GetBalance(ctx context.Context, mobileUserID string) (int64, error)
}
