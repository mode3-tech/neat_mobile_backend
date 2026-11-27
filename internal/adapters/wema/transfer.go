package wema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"neat_mobile_app_backend/internal/modules/tier"
	"neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/providers/baas"
)

const bankListTTL = 6 * time.Hour

// TransferAdapter wraps *baas.Alat so it satisfies
// wallet.TransferProviderService and wallet.WemaProvider.
//
// Debits (ClientTransfer) and payouts (FundWallet) are asynchronous: ALAT
// replies "pending", calls our Authentication Callback with the securityInfo
// we sent, and only then moves the money. The reference we send is always our
// own transaction ID, so the callback and the status sweep can find the row.
type TransferAdapter struct {
	alat   *baas.Alat
	signer *Signer
	now    func() time.Time

	mu        sync.Mutex
	banks     []wallet.Bank
	banksAt   time.Time
	bankNames map[string]string
}

// NewTransferAdapter returns a TransferAdapter. signer signs securityInfo.
func NewTransferAdapter(alat *baas.Alat, signer *Signer) *TransferAdapter {
	return &TransferAdapter{alat: alat, signer: signer, now: time.Now}
}

// FetchBanks lists banks from ALAT (cached, since the list rarely changes and
// every transfer needs it to resolve the bank name).
func (a *TransferAdapter) FetchBanks(ctx context.Context) ([]wallet.Bank, error) {
	a.mu.Lock()
	if a.banks != nil && a.now().Sub(a.banksAt) < bankListTTL {
		banks := a.banks
		a.mu.Unlock()
		return banks, nil
	}
	a.mu.Unlock()

	resp, err := a.alat.GetAllBanks(ctx)
	if err != nil {
		return nil, err
	}
	if err := resultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	banks := make([]wallet.Bank, 0, len(resp.Result))
	names := make(map[string]string, len(resp.Result))
	for _, b := range resp.Result {
		code := strings.TrimSpace(b.BankCode)
		if code == "" {
			continue
		}
		banks = append(banks, wallet.Bank{Code: code, Name: strings.TrimSpace(b.BankName)})
		names[code] = strings.TrimSpace(b.BankName)
	}

	a.mu.Lock()
	a.banks, a.bankNames, a.banksAt = banks, names, a.now()
	a.mu.Unlock()
	return banks, nil
}

func (a *TransferAdapter) bankName(ctx context.Context, code string) string {
	code = strings.TrimSpace(code)
	if code == baas.WemaBankCode {
		return baas.WemaBankName
	}
	if _, err := a.FetchBanks(ctx); err != nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.bankNames[code]
}

// FetchBankDetails is Debit Wallet Step 3 (destination name enquiry).
func (a *TransferAdapter) FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*wallet.BankDetails, error) {
	resp, err := a.alat.AccountNameEnquiry(ctx, strings.TrimSpace(bankCode), strings.TrimSpace(accountNumber))
	if err != nil {
		return nil, err
	}
	if err := resultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, errors.New("wema: name enquiry returned no result")
	}
	r := resp.Result
	code := strings.TrimSpace(r.BankCode)
	if code == "" {
		code = strings.TrimSpace(bankCode)
	}
	return &wallet.BankDetails{BankCode: code, AccountName: r.AccountName, AccountNumber: r.AccountNumber}, nil
}

// GetCustomerDetails returns the wallet's live ALAT balance. ALAT looks
// wallets up by account number, which is what the wallet service passes for
// Wema wallets (see customerLookupKey).
func (a *TransferAdapter) GetCustomerDetails(ctx context.Context, accountNumber string) (*wallet.ProvidusCustomerDetailsResponse, error) {
	d, err := walletDetails(ctx, a.alat, accountNumber)
	if err != nil {
		return nil, err
	}
	return &wallet.ProvidusCustomerDetailsResponse{
		Status: true,
		Customer: wallet.ProvidusCustomer{
			ID:               d.number,
			WalletID:         d.number,
			AccountNumber:    d.number,
			AvailableBalance: d.balance,
			BookedBalance:    d.balance,
		},
	}, nil
}

// InitiateTransfer runs Debit Wallet Steps 1 and 5: it checks we may debit
// the source wallet, then submits ProcessClientTransfer. transferInfo.Amount
// is in kobo (the wallet service converts before calling providers).
func (a *TransferAdapter) InitiateTransfer(ctx context.Context, source wallet.TransferSource, transferInfo *wallet.TransferRequest) (*wallet.TransferResponse, error) {
	if transferInfo == nil {
		return nil, errors.New("wema: transfer request is required")
	}
	ref := internalTransactionID(transferInfo.Metadata)
	if ref == "" {
		return nil, errors.New("wema: transfer has no internal_transaction_id to use as the ALAT reference")
	}
	sourceAccount := strings.TrimSpace(source.AccountNumber)
	if err := walletEnquiry(ctx, a.alat.WalletNameEnquiry, sourceAccount); err != nil {
		return nil, fmt.Errorf("wema: source account enquiry: %w", err)
	}
	securityInfo, err := a.signer.Sign(PurposeSecurityInfo, ref)
	if err != nil {
		return nil, err
	}

	bankCode := strings.TrimSpace(transferInfo.SortCode)
	accountName := ""
	if transferInfo.AccountName != nil {
		accountName = strings.TrimSpace(*transferInfo.AccountName)
	}
	narration := ""
	if transferInfo.Narration != nil {
		narration = strings.TrimSpace(*transferInfo.Narration)
	}
	amountNaira := transferInfo.Amount / 100

	bankName := a.bankName(ctx, bankCode)
	logTransferRequest(ref, sourceAccount, strings.TrimSpace(transferInfo.AccountNumber), bankCode, bankName, accountName, amountNaira)
	resp, err := a.alat.ProcessClientTransfer(ctx, baas.AlatClientTransferRequest{
		SecurityInfo:             securityInfo,
		Amount:                   amountNaira,
		DestinationBankCode:      bankCode,
		DestinationBankName:      bankName,
		DestinationAccountNumber: strings.TrimSpace(transferInfo.AccountNumber),
		DestinationAccountName:   accountName,
		SourceAccountNumber:      sourceAccount,
		Narration:                narration,
		TransactionReference:     ref,
		UseCustomNarration:       narration != "",
	})
	if err != nil {
		return nil, err
	}
	if err := resultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	return transferResponse(resp.Result, ref, amountNaira, strings.TrimSpace(transferInfo.AccountNumber)), nil
}

// logTransferRequest logs what ProcessClientTransfer is about to send, with the
// account numbers masked, so a rejection can be compared with what ALAT
// received. securityInfo is never logged.
func logTransferRequest(ref, source, destination, bankCode, bankName, accountName string, amountNaira float64) {
	log.Printf("[ALAT] level=INFO op=transfer_request ref=%s source=%s destination=%s bank_code=%s bank_name=%q destination_name=%q amount_naira=%.2f",
		ref, maskTail(source), maskTail(destination), bankCode, bankName, accountName, amountNaira)
}

// FundWallet runs Credit Wallet Steps 1 and 2: it checks the destination
// wallet belongs to our channel, then pays amountNaira into it from our
// payout account. reference must be our transaction ID.
func (a *TransferAdapter) FundWallet(ctx context.Context, accountNumber string, amountNaira float64, narration, reference string) (*wallet.TransferResponse, error) {
	accountNumber = strings.TrimSpace(accountNumber)
	if err := walletEnquiry(ctx, a.alat.CreditWalletNameEnquiry, accountNumber); err != nil {
		return nil, fmt.Errorf("wema: destination wallet enquiry: %w", err)
	}
	securityInfo, err := a.signer.Sign(PurposeSecurityInfo, reference)
	if err != nil {
		return nil, err
	}
	narration = strings.TrimSpace(narration)
	resp, err := a.alat.FundWallet(ctx, baas.AlatFundWalletRequest{
		SecurityInfo:             securityInfo,
		DestinationAccountNumber: accountNumber,
		Amount:                   amountNaira,
		Narration:                narration,
		TransactionReference:     reference,
		UseCustomNarration:       narration != "",
	})
	if err != nil {
		return nil, err
	}
	if err := resultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return nil, err
	}
	return transferResponse(resp.Result, reference, amountNaira, accountNumber), nil
}

// CheckDebitStatus is Debit Wallet Step 6.
func (a *TransferAdapter) CheckDebitStatus(ctx context.Context, reference string) (wallet.ProviderTransferStatus, error) {
	return transferStatus(a.alat.ConfirmClientTransferStatus(ctx, reference))
}

// CheckCreditStatus is Credit Wallet Step 3.
func (a *TransferAdapter) CheckCreditStatus(ctx context.Context, reference string) (wallet.ProviderTransferStatus, error) {
	return transferStatus(a.alat.ConfirmFundWalletStatus(ctx, reference))
}

// TransactionHistory is Account Management transhistoryV2.
func (a *TransferAdapter) TransactionHistory(ctx context.Context, accountNumber string, from, to time.Time, keyword string) ([]wallet.ProviderTransaction, error) {
	resp, err := a.alat.GetTransactionHistory(ctx, baas.AlatTransactionHistoryRequest{
		AccountNumber: strings.TrimSpace(accountNumber),
		From:          &from,
		To:            &to,
		KeyWord:       keyword,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Successful {
		return nil, resultError(true, resp.Message, nil)
	}
	return providerTransactions(resp.Result), nil
}

// InitiateStatement is Get Statement InitiateGetCustomerStatement.
func (a *TransferAdapter) InitiateStatement(ctx context.Context, accountNumber string, from, to time.Time) (string, error) {
	resp, err := a.alat.InitiateCustomerStatement(ctx, baas.AlatInitiateStatementRequest{
		AccountNumber: strings.TrimSpace(accountNumber),
		DateFrom:      from.UTC(),
		DateTo:        to.UTC(),
	})
	if err != nil {
		return "", err
	}
	if !resp.Status {
		return "", resultError(true, resp.Message, nil)
	}
	ref := ""
	if resp.Data != nil {
		ref = strings.TrimSpace(resp.Data.ReferenceID)
	}
	if ref == "" {
		return "", errors.New("wema: statement request returned no reference ID")
	}
	return ref, nil
}

// StatementTransactions is Get Statement GetCustomerTransactions.
func (a *TransferAdapter) StatementTransactions(ctx context.Context, referenceID string) ([]wallet.ProviderTransaction, error) {
	resp, err := a.alat.GetCustomerTransactions(ctx, baas.AlatCustomerTransactionsRequest{ReferenceID: referenceID})
	if err != nil {
		return nil, err
	}
	if !resp.Status {
		return nil, resultError(true, resp.Message, nil)
	}
	return providerTransactions(resp.Data), nil
}

// AccountFinder satisfies account.CustomerAccountFinder for Wema wallets,
// keyed by account number.
type AccountFinder struct {
	alat *baas.Alat
}

func NewAccountFinder(alat *baas.Alat) *AccountFinder {
	return &AccountFinder{alat: alat}
}

func (f *AccountFinder) GetCustomerDetails(ctx context.Context, accountNumber string) (*baas.ProvidusCustomerDetailsResponse, error) {
	d, err := walletDetails(ctx, f.alat, accountNumber)
	if err != nil {
		return nil, err
	}
	var resp baas.ProvidusCustomerDetailsResponse
	resp.Status = true
	resp.Customer.ID = d.number
	resp.Customer.WalletID = d.number
	resp.Customer.AccountNumber = d.number
	resp.Customer.AvailableBalance = d.balance
	resp.Customer.BookedBalance = d.balance
	return &resp, nil
}

// TierUpgrader satisfies tier.WemaTierUpgrader.
type TierUpgrader struct {
	alat *baas.Alat
}

func NewTierUpgrader(alat *baas.Alat) *TierUpgrader {
	return &TierUpgrader{alat: alat}
}

func (u *TierUpgrader) UpgradeToTier2(ctx context.Context, accountNumber, nin, bvn, liveImageOfFace string) error {
	resp, err := u.alat.UpgradeToTier2(ctx, baas.AlatTier2UpgradeRequest{
		AccountNumber:   accountNumber,
		NIN:             nin,
		BVN:             bvn,
		LiveImageOfFace: liveImageOfFace,
	})
	if err != nil {
		return err
	}
	return envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors)
}

func (u *TierUpgrader) UpgradeToTier3(ctx context.Context, accountNumber string, addr tier.ResidentialAddress) error {
	resp, err := u.alat.UpgradeToTier3(ctx, baas.AlatTier3UpgradeRequest{
		AccountNumber: accountNumber,
		ResidentialAddress: baas.ResidentialAddress{
			BuildingNumber:        addr.BuildingNumber,
			Apartment:             addr.Apartment,
			Street:                addr.Street,
			City:                  addr.City,
			Town:                  addr.Town,
			State:                 addr.State,
			LGA:                   addr.Lga,
			LCDA:                  addr.Lcda,
			Landmark:              addr.Landmark,
			AdditionalInformation: addr.AdditionalInformation,
			Country:               addr.Country,
			FullAddress:           addr.FullAddress,
			PostalCode:            addr.PostalCode,
		},
	})
	if err != nil {
		return err
	}
	return envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors)
}

func (u *TierUpgrader) KYCStatus(ctx context.Context, accountNumber string) (json.RawMessage, error) {
	resp, err := u.alat.GetKYCStatus(ctx, accountNumber)
	if err != nil {
		return nil, err
	}
	if err := envelopeError(resp.Status, resp.Message, resp.Code, resp.StatusCode, resp.Errors); err != nil {
		return nil, err
	}
	return json.Marshal(resp.Data) // nil Data marshals to null
}

type alatWalletDetails struct {
	number  string
	balance float64 // naira
}

func walletDetails(ctx context.Context, alat *baas.Alat, accountNumber string) (*alatWalletDetails, error) {
	resp, err := alat.GetWalletDetailsWithAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, err
	}
	if !resp.Successful || resp.Result == nil {
		return nil, resultError(true, resp.Message, nil)
	}
	balance, err := parseNaira(resp.Result.AvailableBalance)
	if err != nil {
		return nil, fmt.Errorf("wema: unreadable available balance %q: %w", resp.Result.AvailableBalance, err)
	}
	number := strings.TrimSpace(resp.Result.WalletNumber)
	if number == "" {
		number = strings.TrimSpace(accountNumber)
	}
	return &alatWalletDetails{number: number, balance: balance}, nil
}

// parseNaira reads ALAT's string balance ("1,000.50", "NGN 1000.50").
func parseNaira(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "NGN"), "₦")
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0, errors.New("empty balance")
	}
	return strconv.ParseFloat(s, 64)
}

func internalTransactionID(metadata map[string]any) string {
	if id, ok := metadata["internal_transaction_id"].(string); ok {
		return strings.TrimSpace(id)
	}
	return ""
}

// transferResponse maps ALAT's (usually "pending") transfer result. Only an
// explicit failure is unsuccessful; anything else is left for the status
// sweep to settle. ALAT pays NIP charges itself, so there are none to record.
func transferResponse(r *baas.AlatTransferResult, ref string, amountNaira float64, destination string) *wallet.TransferResponse {
	if r == nil {
		r = &baas.AlatTransferResult{} // no result yet: pending, left to the callback and sweep
	}
	message := strings.TrimSpace(r.Message)
	if message == "" {
		message = strings.TrimSpace(r.Status)
	}
	return &wallet.TransferResponse{
		Status:  normaliseStatus(r.Status) != wallet.ProviderTransferFailed,
		Message: message,
		Transfer: wallet.TransferResult{
			Amount:               amountNaira,
			Total:                amountNaira,
			Reference:            ref,
			TransactionReference: ref,
			SessionID:            strings.TrimSpace(r.PlatformTransactionReference),
			Destination:          destination,
			Description:          message,
		},
	}
}

// transferStatus maps a ConfirmClientTransferStatus reply. Anything short of
// a clear final status stays pending, to be asked again later.
func transferStatus(resp *baas.AlatTransferStatusResponse, err error) (wallet.ProviderTransferStatus, error) {
	if err != nil {
		return wallet.ProviderTransferPending, err
	}
	if err := resultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return wallet.ProviderTransferPending, err
	}
	if resp.Result == nil || resp.Result.Data == nil {
		return wallet.ProviderTransferPending, nil
	}
	return normaliseStatus(resp.Result.Data.Status), nil
}

// walletEnquiry runs a wallet name enquiry and fails if ALAT doesn't confirm
// the wallet.
func walletEnquiry(ctx context.Context, enquire func(context.Context, string) (*baas.AlatWalletNameEnquiryResponse, error), accountNumber string) error {
	resp, err := enquire(ctx, accountNumber)
	if err != nil {
		return err
	}
	if err := resultError(resp.HasError, resp.ErrorMessage, resp.ErrorMessages); err != nil {
		return err
	}
	if resp.Result == nil {
		return errors.New("wema: wallet enquiry returned no result")
	}
	return nil
}

// normaliseStatus maps ALAT's free-text transfer status. Its values aren't
// enumerated in the docs, so unknown values stay pending (and get re-polled)
// rather than being guessed as final.
func normaliseStatus(status string) wallet.ProviderTransferStatus {
	s := strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.Contains(s, "fail"), strings.Contains(s, "revers"), strings.Contains(s, "declin"),
		strings.Contains(s, "reject"), strings.Contains(s, "cancel"),
		strings.Contains(s, "unsuccess"), strings.Contains(s, "not successful"):
		return wallet.ProviderTransferFailed
	case strings.Contains(s, "success"), s == "completed", s == "complete":
		return wallet.ProviderTransferSuccessful
	default:
		return wallet.ProviderTransferPending
	}
}

func providerTransactions(items []baas.AlatTransactionHistory) []wallet.ProviderTransaction {
	out := make([]wallet.ProviderTransaction, len(items))
	for i, it := range items {
		out[i] = wallet.ProviderTransaction{
			Title:                    it.Title,
			Amount:                   it.Amount,
			Type:                     string(it.Type),
			Date:                     it.Date,
			TransactionDate:          it.TransactionDate,
			Narration:                it.Narration,
			Status:                   string(it.Status),
			CreditType:               string(it.CreditType),
			Sender:                   it.Sender,
			SenderAccountNumber:      it.SenderAccountNumber,
			DestinationBank:          it.DestinationBank,
			DestinationAccountNumber: it.DestinationAccountNumber,
			ReceiverName:             it.ReceiverName,
			ReferenceID:              it.ReferenceID,
			IsViewReceiptEnabled:     it.IsViewReceiptEnabled,
			TranID:                   it.TranID,
		}
	}
	return out
}

// Compile-time interface checks.
var (
	_ wallet.TransferProviderService = (*TransferAdapter)(nil)
	_ wallet.WemaProvider            = (*TransferAdapter)(nil)
	_ tier.WemaTierUpgrader          = (*TierUpgrader)(nil)
)
