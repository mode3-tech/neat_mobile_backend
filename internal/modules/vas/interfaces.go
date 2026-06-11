package vas

import (
	"context"
	"neat_mobile_app_backend/providers/baas"
	vasprovider "neat_mobile_app_backend/providers/vas"
)

type VASService interface {
	FetchAllCategories(ctx context.Context) (*vasprovider.CategoriesResponse, error)
	FetchBillersByCategoryID(ctx context.Context, categoryID, page, size int) (*vasprovider.BillersByCategoryIDResponse, error)
	FetchProductsByCategoryIDAndBillerID(ctx context.Context, categoryID, billerID, page, size int) (*vasprovider.ProductResponse, error)
	GetAirtime(ctx context.Context, requestID, uniqueCode, phoneNumber string, amount int64) (*vasprovider.ISPResponse, error)
	GetData(ctx context.Context, requestID, uniqueCode, phoneNumber string, amount int64) (*vasprovider.ISPResponse, error)
	ValidateElectricity(ctx context.Context, requestID, uniqueCode, accountNumber string, accountType vasprovider.AccountType) (*vasprovider.ElectricityValidationResponse, error)
	PayElectricityBill(ctx context.Context, requestID, uniqueCode, accountNumber, name, address, phoneNumber string, accountType vasprovider.AccountType, amount int64) (*vasprovider.PayElectricityResponse, error)
	ValidateCable(ctx context.Context, requestID, uniqueCode, accountNumber string, noOfMonth int) (*vasprovider.CableValidationResponse, error)
	PayCableBill(ctx context.Context, requestID, uniqueCode, accountNumber, accountType, name, phoneNumber string, noOfMonth int, amount int64) (*vasprovider.PayCableResponse, error)
}

type WalletService interface {
	GetBalance(ctx context.Context, mobileUserID string) (*CustomerWallet, error)
}

type TransactionService interface {
	AddTransaction(ctx context.Context, transaction *Transaction) error
	UpdateTransactionStatus(ctx context.Context, txID string, balanceAfter int64, status TransactionStatus) error
}

type BAAS interface {
	DebitCustomer(ctx context.Context, amount int64, customerID, referenceID string, metadata interface{}) (*baas.ProvidusWalletDebitResponse, error)
	CreditCustomer(ctx context.Context, amount int64, referenceID, customerID string, metadata interface{}) (*baas.ProvidusWalletCreditResponse, error)
}

type AuthService interface {
	VerifyTransactionPin(ctx context.Context, mobileUserID, pin string) error
}
