// Package providus adapts the concrete *baas.Providus client to the wallet
// package's TransferProviderService interface. The wallet package defines its
// own local DTOs so it does not import providers/baas; importing it would form
// the cycle wallet -> baas -> auth -> wallet. This adapter is the single place
// that depends on both packages and converts between their (identical) DTOs.
package providus

import (
	"context"

	"neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/providers/baas"
)

// Adapter wraps *baas.Providus so it satisfies wallet.TransferProviderService.
type Adapter struct {
	p *baas.Providus
}

// New returns an Adapter that implements wallet.TransferProviderService.
func New(p *baas.Providus) *Adapter {
	return &Adapter{p: p}
}

func (a *Adapter) FetchBanks(ctx context.Context) ([]wallet.Bank, error) {
	banks, err := a.p.FetchBanks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]wallet.Bank, len(banks))
	for i, b := range banks {
		out[i] = wallet.Bank(b)
	}
	return out, nil
}

func (a *Adapter) FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*wallet.BankDetails, error) {
	d, err := a.p.FetchBankDetails(ctx, accountNumber, bankCode)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	wd := wallet.BankDetails(*d)
	return &wd, nil
}

func (a *Adapter) GetCustomerDetails(ctx context.Context, customerID string) (*wallet.ProvidusCustomerDetailsResponse, error) {
	res, err := a.p.GetCustomerDetails(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return &wallet.ProvidusCustomerDetailsResponse{
		Status:   res.Status,
		Customer: wallet.ProvidusCustomer(res.Customer),
	}, nil
}

func (a *Adapter) InitiateTransfer(ctx context.Context, source wallet.TransferSource, transferInfo *wallet.TransferRequest) (*wallet.TransferResponse, error) {
	var req *baas.TransferRequest
	if transferInfo != nil {
		r := baas.TransferRequest(*transferInfo)
		req = &r
	}
	res, err := a.p.InitiateTransfer(ctx, source.WalletCustomerID, req)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return &wallet.TransferResponse{
		Status:   res.Status,
		Message:  res.Message,
		Transfer: wallet.TransferResult(res.Transfer),
	}, nil
}
