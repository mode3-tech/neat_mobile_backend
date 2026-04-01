package wallet

import "context"

type BankResponse struct {
	Status bool   `json:"status"`
	Banks  []Bank `json:"banks"`
}

type Bank struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type BankDetails struct {
	BankCode      string `json:"bankCode"`
	AccountName   string `json:"accountName"`
	AccountNumber string `json:"accountNumber"`
}

type ProvidusService interface {
	FetchBanks(ctx context.Context) ([]Bank, error)
	FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*BankDetails, error)
	InitiateTransfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error)
}
