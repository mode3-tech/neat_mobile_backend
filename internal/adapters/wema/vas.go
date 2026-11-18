package wema

import (
	"context"
	"errors"
	"strings"

	"neat_mobile_app_backend/internal/modules/vas"
	"neat_mobile_app_backend/providers/baas"
)

func (a *TransferAdapter) GetWalletBalance(ctx context.Context, accountNumber string) (float64, error) {
	details, err := walletDetails(ctx, a.alat, accountNumber)
	if err != nil {
		return 0, err
	}
	return details.balance, nil
}

func (a *TransferAdapter) GetBills(ctx context.Context) (*baas.AlatBillsResponse, error) {
	resp, err := a.alat.GetAllBills(ctx)
	if err != nil {
		return nil, err
	}
	if err := baas.AlatResultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	return resp, nil
}

func (a *TransferAdapter) GetDataPlans(ctx context.Context) (*baas.AlatDataPlansResponse, error) {
	resp, err := a.alat.GetDataPlans(ctx)
	if err != nil {
		return nil, err
	}
	if err := baas.AlatResultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	return resp, nil
}

func (a *TransferAdapter) ValidateBill(ctx context.Context, identifier string, packageID int) (*baas.AlatBillValidationResponse, error) {
	resp, err := a.alat.ValidateBill(ctx, baas.AlatBillValidationRequest{
		Identifier: strings.TrimSpace(identifier),
		PackageID:  packageID,
	})
	if err != nil {
		return nil, err
	}
	if err := baas.AlatResultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, errors.New("wema: bill validation returned no result")
	}
	return resp, nil
}

func (a *TransferAdapter) PurchaseAirtime(ctx context.Context, accountNumber, network, phoneNumber string, amount float64, reference string) (*baas.AlatClientVASResponse, error) {
	securityInfo, err := a.signer.Sign(PurposeSecurityInfo, reference)
	if err != nil {
		return nil, err
	}
	resp, err := a.alat.PurchaseClientAirtime(ctx, baas.AlatClientAirtimeRequest{
		TransactionReference: reference,
		AccountNumber:        strings.TrimSpace(accountNumber),
		Network:              strings.TrimSpace(network),
		PhoneNumber:          strings.TrimSpace(phoneNumber),
		Amount:               amount,
		SecurityInfo:         securityInfo,
	})
	if err != nil {
		return nil, err
	}
	if err := baas.AlatResultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	return resp, nil
}

func (a *TransferAdapter) PurchaseData(ctx context.Context, accountNumber, network, phoneNumber string, packageCode int, amount float64, reference string) (*baas.AlatClientVASResponse, error) {
	securityInfo, err := a.signer.Sign(PurposeSecurityInfo, reference)
	if err != nil {
		return nil, err
	}
	resp, err := a.alat.PurchaseClientData(ctx, baas.AlatClientDataRequest{
		TransactionReference: reference,
		AccountNumber:        strings.TrimSpace(accountNumber),
		PhoneNumber:          strings.TrimSpace(phoneNumber),
		PackageCode:          packageCode,
		Amount:               amount,
		Network:              strings.TrimSpace(network),
		SecurityInfo:         securityInfo,
	})
	if err != nil {
		return nil, err
	}
	if err := baas.AlatResultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	return resp, nil
}

func (a *TransferAdapter) PayBill(ctx context.Context, accountNumber, customerIdentifier, customerEmail, customerPhone, customerName string, packageID int, amount, charge float64, reference string) (*baas.AlatClientVASResponse, error) {
	securityInfo, err := a.signer.Sign(PurposeSecurityInfo, reference)
	if err != nil {
		return nil, err
	}
	resp, err := a.alat.PayClientBill(ctx, baas.AlatClientBillRequest{
		CustomerAccount:      strings.TrimSpace(accountNumber),
		Amount:               amount,
		Charge:               charge,
		TransactionReference: reference,
		PackageID:            packageID,
		CustomerIdentifier:   strings.TrimSpace(customerIdentifier),
		CustomerEmail:        strings.TrimSpace(customerEmail),
		CustomerPhoneNumber:  strings.TrimSpace(customerPhone),
		CustomerName:         strings.TrimSpace(customerName),
		SecurityInfo:         securityInfo,
	})
	if err != nil {
		return nil, err
	}
	if err := baas.AlatResultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	return resp, nil
}

var _ vas.WemaVASProvider = (*TransferAdapter)(nil)
