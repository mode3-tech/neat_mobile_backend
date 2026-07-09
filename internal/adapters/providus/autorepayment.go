package providus

import (
	"context"

	"neat_mobile_app_backend/internal/modules/autorepayment"
	"neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/providers/baas"
)

// AutoRepaymentAdapter wraps *baas.Providus so it satisfies
// autorepayment.ProvidusService. Like Adapter, it converts baas DTOs to the
// consumer's local DTOs so the autorepayment package never imports
// providers/baas. Transfers use the wallet DTOs the interface references.
type AutoRepaymentAdapter struct {
	p *baas.Providus
}

// NewAutoRepayment returns an adapter implementing autorepayment.ProvidusService.
func NewAutoRepayment(p *baas.Providus) *AutoRepaymentAdapter {
	return &AutoRepaymentAdapter{p: p}
}

func (a *AutoRepaymentAdapter) FetchBanks(ctx context.Context) ([]autorepayment.Bank, error) {
	banks, err := a.p.FetchBanks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]autorepayment.Bank, len(banks))
	for i, b := range banks {
		out[i] = autorepayment.Bank(b)
	}
	return out, nil
}

func (a *AutoRepaymentAdapter) FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*autorepayment.BankDetails, error) {
	d, err := a.p.FetchBankDetails(ctx, accountNumber, bankCode)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	ad := autorepayment.BankDetails(*d)
	return &ad, nil
}

func (a *AutoRepaymentAdapter) GetCustomerDetails(ctx context.Context, customerID string) (*autorepayment.ProvidusCustomerDetailsResponse, error) {
	res, err := a.p.GetCustomerDetails(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return &autorepayment.ProvidusCustomerDetailsResponse{
		Status:   res.Status,
		Customer: autorepayment.ProvidusCustomer(res.Customer),
	}, nil
}

func (a *AutoRepaymentAdapter) InitiateTransfer(ctx context.Context, providusCustomerID string, transferInfo *wallet.TransferRequest) (*wallet.TransferResponse, error) {
	var req *baas.TransferRequest
	if transferInfo != nil {
		r := baas.TransferRequest(*transferInfo)
		req = &r
	}
	res, err := a.p.InitiateTransfer(ctx, providusCustomerID, req)
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
