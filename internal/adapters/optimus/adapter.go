// Package optimus adapts the concrete *baas.Optimus client to the wallet
// package's TransferProviderService interface, mirroring
// internal/adapters/providus/adapter.go's role and reasoning: the wallet
// package defines its own local DTOs so it does not import providers/baas
// (which would form the cycle wallet -> baas -> auth -> wallet), and this
// adapter is the one place that depends on both packages.
package optimus

import (
	"context"
	"fmt"
	"strings"

	"neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/providers/baas"

	"github.com/google/uuid"
)

// isIntrabank reports whether a transfer to beneficiaryBankCode stays within
// the same bank as the source account (sourceBankCode) - per
// docs/README(1).md, this is what decides whether SessionId must be left
// empty (intrabank) or populated from Name Enquiry (interbank), not a
// separate endpoint. An empty beneficiary bank code is never intrabank.
func isIntrabank(beneficiaryBankCode, sourceBankCode string) bool {
	beneficiaryBankCode = strings.TrimSpace(beneficiaryBankCode)
	if beneficiaryBankCode == "" {
		return false
	}
	return strings.EqualFold(beneficiaryBankCode, strings.TrimSpace(sourceBankCode))
}

// Adapter wraps *baas.Optimus so it satisfies wallet.TransferProviderService.
type Adapter struct {
	o *baas.Optimus
}

// New returns an Adapter that implements wallet.TransferProviderService.
func New(o *baas.Optimus) *Adapter {
	return &Adapter{o: o}
}

func (a *Adapter) FetchBanks(ctx context.Context) ([]wallet.Bank, error) {
	banks, err := a.o.FetchBanks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]wallet.Bank, len(banks))
	for i, b := range banks {
		out[i] = wallet.Bank{Code: b.BankCode, Name: b.BankName}
	}
	return out, nil
}

func (a *Adapter) FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*wallet.BankDetails, error) {
	data, err := a.o.NameEnquiry(ctx, accountNumber, bankCode)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	return &wallet.BankDetails{
		BankCode:      data.BankCode,
		AccountName:   data.AccountName,
		AccountNumber: data.AccountNumber,
	}, nil
}

// GetCustomerDetails has no Optimus equivalent today (no balance/customer
// lookup endpoint has been integrated - see the Non-Goals section of the
// transfer implementation plan). Callers that need a balance check before
// transferring must skip it for Optimus-sourced wallets rather than call this.
func (a *Adapter) GetCustomerDetails(ctx context.Context, customerID string) (*wallet.ProvidusCustomerDetailsResponse, error) {
	return nil, fmt.Errorf("optimus: GetCustomerDetails is not supported")
}

// InitiateTransfer submits an intrabank or interbank transfer. It always
// performs a fresh Name Enquiry immediately beforehand to get a current
// SessionID (Optimus session IDs are assumed short-lived/one-time, like its
// BVN OTP reference IDs elsewhere in this codebase - a SessionID surfaced by
// an earlier, client-driven FetchBankDetails call can't be trusted to still
// be valid), then decides intrabank vs interbank by comparing the
// beneficiary's bank code against the source wallet's own bank code
// (source.BankCode) - per docs/README(1).md, intrabank transfers must leave
// SessionId empty even though Name Enquiry returns one regardless.
//
// No local balance check is performed - Optimus's transfer endpoint is the
// source of truth on sufficiency and will reject the request itself.
func (a *Adapter) InitiateTransfer(ctx context.Context, source wallet.TransferSource, transferInfo *wallet.TransferRequest) (*wallet.TransferResponse, error) {
	if transferInfo == nil {
		return nil, fmt.Errorf("optimus: transfer request is required")
	}

	beneficiaryBankCode := strings.TrimSpace(transferInfo.SortCode)
	beneficiaryAccount := strings.TrimSpace(transferInfo.AccountNumber)

	enquiry, err := a.o.NameEnquiry(ctx, beneficiaryAccount, beneficiaryBankCode)
	if err != nil {
		return nil, fmt.Errorf("optimus: name enquiry before transfer: %w", err)
	}

	sessionID := ""
	if !isIntrabank(beneficiaryBankCode, source.BankCode) && enquiry != nil {
		sessionID = enquiry.SessionID
	}

	narration := ""
	if transferInfo.Narration != nil {
		narration = *transferInfo.Narration
	}

	req := &baas.OptimusTransferRequest{
		RequestId:            uuid.NewString(),
		TransactionReference: uuid.NewString(),
		Amount:               transferInfo.Amount / 100, // kobo -> naira, matching the Providus adapter's conversion
		Narration:            narration,
		SourceAccount:        strings.TrimSpace(source.AccountNumber),
		BeneficiaryAccount:   beneficiaryAccount,
		BeneficiaryBankCode:  beneficiaryBankCode,
		SessionId:            sessionID,
	}

	resp, err := a.o.InitiateTransfer(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Data) == 0 {
		message := "optimus returned an empty transfer response"
		if resp != nil && strings.TrimSpace(resp.ResponseMessage) != "" {
			message = resp.ResponseMessage
		}
		return &wallet.TransferResponse{Status: false, Message: message}, nil
	}

	result := resp.Data[0]
	return &wallet.TransferResponse{
		Status:  true,
		Message: result.ResponseMessage,
		Transfer: wallet.TransferResult{
			Amount:               result.TransactionAmount,
			Reference:            result.TransactionReference,
			Total:                result.TransactionAmount,
			SessionID:            result.SessionID,
			Destination:          result.AccountCredited,
			TransactionReference: result.TransactionReference,
			Description:          result.ResponseMessage,
		},
	}, nil
}
