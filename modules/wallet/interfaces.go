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

type ProvidusService interface {
	FetchBanks(ctx context.Context) ([]Bank, error)
}
