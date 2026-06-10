package wallet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/modules/transaction"
	"neat_mobile_app_backend/internal/pinverifier"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SettlementAccount struct {
	AccountNumber string
	BankCode      string
	AccountName   string
}

type Service struct {
	repo              *Repository
	providusService   ProvidusService
	pinVerifier       *pinverifier.Verifier
	settlementAccount SettlementAccount
	deviceVerifier    DeviceVerifier
}

func NewService(repo *Repository, providusService ProvidusService, pinVerifier *pinverifier.Verifier, settlementAccount SettlementAccount, deviceVerifier DeviceVerifier) *Service {
	return &Service{
		repo:              repo,
		providusService:   providusService,
		pinVerifier:       pinVerifier,
		settlementAccount: settlementAccount,
		deviceVerifier:    deviceVerifier,
	}
}

func (s *Service) FetchBanks(ctx context.Context) ([]Bank, error) {
	banks, err := s.providusService.FetchBanks(ctx)
	if err != nil {
		return nil, appErr.ErrFetchingBanks
	}

	return banks, nil
}

func (s *Service) FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*BankDetails, error) {
	bankDetails, err := s.providusService.FetchBankDetails(ctx, accountNumber, bankCode)
	if err != nil {
		return nil, appErr.ErrFetchingBankDetails
	}

	return bankDetails, nil
}

func (s *Service) InitiateTransfer(ctx context.Context, mobileUserID string, req *TransferRequest) (*TransferResponse, error) {
	if err := s.pinVerifier.Verify(ctx, mobileUserID, req.TransactionPin); err != nil {
		return nil, err
	}

	if req.Amount <= 50 {
		return nil, appErr.ErrInvalidTransferAmount
	}
	req.Amount = req.Amount * 100 // convert Naira → kobo for storage and downstream use

	user, err := s.repo.GetUserByMobileUserID(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrUnauthorized
		}

		return nil, appErr.ErrFundsTransfer
	}

	if user.CreatedAt.Add(24*time.Hour).After(time.Now()) && req.Amount > 20000*100 {
		return nil, appErr.ErrNewUserTransferRestriction
	}

	accountNumber := strings.TrimSpace(req.AccountNumber)
	accountName := ""
	if req.AccountName != nil {
		accountName = strings.TrimSpace(*req.AccountName)
	}

	if accountNumber == "" {
		return nil, appErr.ErrInvalidRequestBody
	}

	if accountName == "" {
		return nil, appErr.ErrInvalidRequestBody
	}
	walletUser, err := s.repo.GetUserWalletID(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrFundsTransfer
	}

	narration := ""
	if req.Narration != nil {
		narration = *req.Narration
	}

	wallet, err := s.repo.GetWallet(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrMissingUserWallet
		}
		return nil, appErr.ErrFundsTransfer
	}

	txID := uuid.NewString()
	txRecord := &transaction.Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            walletUser.WalletID,
		Category:            transaction.TransactionCategoryTransferTo,
		Type:                transaction.TransactionTypeDebit,
		Description:         fmt.Sprintf("Transfer to %s", accountName),
		Amount:              req.Amount,
		Reference:           uuid.NewString(),
		Narration:           &narration,
		CounterpartyAccount: accountNumber,
		CounterpartyName:    accountName,
		CounterpartyBank:    req.SortCode,
		Source:              "transfer",
		Status:              transaction.TransactionStatusPending,
	}

	if err := s.repo.AddTransaction(ctx, txRecord); err != nil {
		return nil, appErr.ErrFundsTransfer
	}

	resp, err := s.providusService.InitiateTransfer(ctx, wallet.WalletCustomerID, req)
	if err != nil {
		_ = s.repo.UpdateTransactionStatus(ctx, txID, transaction.TransactionStatusFailed)
		return nil, appErr.ErrFundsTransfer
	}

	if resp == nil {
		_ = s.repo.UpdateTransactionStatus(ctx, txID, transaction.TransactionStatusFailed)
		return nil, appErr.ErrFundsTransfer
	}

	if !resp.Status {
		_ = s.repo.UpdateTransactionStatus(ctx, txID, transaction.TransactionStatusFailed)
		message := strings.TrimSpace(resp.Message)
		if message == "" {
			message = "provider returned an unsuccessful transfer response"
		}
		return nil, appErr.ErrFundsTransfer
	}

	totalDebit := req.Amount + int64(math.Round(resp.Transfer.Charges*100)) + int64(math.Round(resp.Transfer.Vat*100))

	if err := s.repo.CompleteDebitTransaction(ctx, txID, resp.Transfer.TransactionReference, transaction.TransactionStatusSuccessful, walletUser.WalletID, totalDebit); err != nil {
		return nil, appErr.ErrFundsTransfer
	}

	return resp, nil

}

func (s *Service) TransferForLoanRepayment(ctx context.Context, mobileUserID string, amountNaira int64) error {
	if amountNaira <= 50 {
		return appErr.ErrInvalidTransferAmount
	}
	if strings.TrimSpace(s.settlementAccount.AccountNumber) == "" {
		return errors.New("loan repayment settlement account is not configured")
	}

	w, err := s.repo.GetWallet(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appErr.ErrMissingUserWallet
		}
		return appErr.ErrMakingLoanRepayment
	}

	amountKobo := amountNaira * 100
	if w.AvailableBalance < amountKobo {
		log.Println("insufficient balance")
		return appErr.ErrInsufficientBalance
	}

	narration := "Loan repayment"
	accountName := s.settlementAccount.AccountName
	txID := uuid.NewString()
	txRecord := &transaction.Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            w.InternalWalletID,
		Type:                transaction.TransactionTypeDebit,
		Category:            transaction.TransactionCategoryLoanRepayment,
		Source:              transaction.TransactionSourceLoanRepayment,
		Amount:              amountKobo,
		Reference:           uuid.NewString(),
		Narration:           &narration,
		CounterpartyAccount: s.settlementAccount.AccountNumber,
		CounterpartyName:    accountName,
		CounterpartyBank:    s.settlementAccount.BankCode,
		Status:              transaction.TransactionStatusPending,
	}
	if err := s.repo.AddTransaction(ctx, txRecord); err != nil {
		return fmt.Errorf("failed to create transaction record: %w", err)
	}

	resp, err := s.providusService.InitiateTransfer(ctx, w.WalletCustomerID, &TransferRequest{
		Amount:        amountNaira,
		SortCode:      s.settlementAccount.BankCode,
		AccountNumber: s.settlementAccount.AccountNumber,
		AccountName:   &accountName,
		Narration:     &narration,
	})
	if err != nil {
		_ = s.repo.UpdateTransactionStatus(ctx, txID, transaction.TransactionStatusFailed)
		return fmt.Errorf("%w: %v", ErrTransferProviderFailed, err)
	}
	if resp == nil || !resp.Status {
		_ = s.repo.UpdateTransactionStatus(ctx, txID, transaction.TransactionStatusFailed)
		msg := "provider returned an unsuccessful transfer response"
		if resp != nil && strings.TrimSpace(resp.Message) != "" {
			msg = resp.Message
		}
		return fmt.Errorf("%w: %s", ErrTransferProviderFailed, msg)
	}

	totalDebit := amountKobo + int64(math.Round(resp.Transfer.Charges*100)) + int64(math.Round(resp.Transfer.Vat*100))
	return s.repo.CompleteDebitTransaction(ctx, txID, resp.Transfer.TransactionReference,
		transaction.TransactionStatusSuccessful, w.InternalWalletID, totalDebit)
}

func (s *Service) InitiateBulkTransfer(ctx context.Context, mobileUserID string, req *BulkTransferRequest) (*BulkTransferResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return nil, fmt.Errorf("%w: mobile user ID is required", ErrInvalidTransferRequest)
	}

	if req == nil || len(req.RecipientInfo) == 0 {
		return nil, fmt.Errorf("%w: at least one recipient is required", ErrInvalidTransferRequest)
	}

	if s.providusService == nil {
		return nil, errors.New("transfer service is not configured")
	}

	if err := s.pinVerifier.Verify(ctx, mobileUserID, req.TransactionPin); err != nil {
		return nil, err
	}

	resp, err := s.providusService.InitiateBulkTransfer(ctx, req.RecipientInfo)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransferProviderFailed, err)
	}

	if resp == nil {
		return nil, errors.New("transfer service returned no response body")
	}

	toResults := func(src []ProvidusBatchTransferResult) []BulkTransferResult {
		out := make([]BulkTransferResult, len(src))
		for i, r := range src {
			out[i] = BulkTransferResult{
				Amount:        r.Amount,
				VAT:           r.VAT,
				SortCode:      r.SortCode,
				Reference:     r.Reference,
				Narration:     r.Narration,
				AccountName:   r.AccountName,
				Fee:           r.Fee,
				AccountNumber: r.AccountNumber,
				Total:         r.Total,
			}
		}
		return out
	}

	return &BulkTransferResponse{
		Status:  "success",
		Message: resp.Message,
		Data: struct {
			All      []BulkTransferResult `json:"all"`
			Rejected []BulkTransferResult `json:"rejected"`
			Accepted []BulkTransferResult `json:"accepted"`
		}{
			All:      toResults(resp.Data.All),
			Rejected: toResults(resp.Data.Rejected),
			Accepted: toResults(resp.Data.Accepted),
		},
	}, nil
}

func (s *Service) AddBeneficiary(ctx context.Context, mobileUserID string, req *AddBeneficiaryRequest) (*Beneficiary, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)

	user, err := s.repo.GetUserWalletID(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrUnauthorized
		}
		return nil, appErr.ErrAddingBeneficiary
	}

	walletID := strings.TrimSpace(user.WalletID)
	accountNumber := strings.TrimSpace(req.AccountNumber)
	accountName := strings.TrimSpace(req.AccountName)
	bankCode := strings.TrimSpace(req.BankCode)

	if mobileUserID == "" {
		return nil, appErr.ErrInvalidRequestBody
	}

	if walletID == "" {
		return nil, appErr.ErrInvalidRequestBody
	}

	if accountNumber == "" {
		return nil, appErr.ErrInvalidRequestBody
	}

	if accountName == "" {
		return nil, appErr.ErrInvalidRequestBody
	}

	if bankCode == "" {
		return nil, appErr.ErrInvalidRequestBody
	}

	beneficiary := &Beneficiary{
		ID:            uuid.NewString(),
		MobileUserID:  mobileUserID,
		WalletID:      walletID,
		BankCode:      bankCode,
		AccountNumber: accountNumber,
		AccountName:   accountName,
	}

	if err := s.repo.CreateBeneficiary(ctx, beneficiary); err != nil {
		return nil, appErr.ErrAddingBeneficiary
	}

	return beneficiary, nil
}

func (s *Service) InitiateDeposit(ctx context.Context, deviceID, mobileUserID string, req InitiatedDepositRequest) (*InitiatedDepositResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return nil, errors.New("invalid mobile user id")
	}

	_, err := s.repo.GetDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify device: %s", err.Error())
	}

	wallet, err := s.repo.GetWallet(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("wallet not found")
		}
		return nil, fmt.Errorf("error fetching wallet: %s", err.Error())
	}

	trackingID := uuid.NewString()
	now := time.Now().UTC()
	expiresAt := now.Add(30 * time.Minute)

	expectedDeposit := &ExpectedDeposit{
		ID:             uuid.NewString(),
		TrackingID:     trackingID,
		MobileUserID:   mobileUserID,
		ExpectedAmount: req.ExpectedAmount,
		WalletID:       wallet.InternalWalletID,
		Status:         ExpectedDepositStatusPending,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
	}

	if err := s.repo.CreateExpectedDeposit(ctx, expectedDeposit); err != nil {
		return nil, errors.New("could not create deposit")
	}

	account := &AccountObj{
		AccountNumber: wallet.AccountNumber,
		AccountName:   wallet.AccountName,
		BankName:      wallet.BankName,
		BankCode:      wallet.BankCode,
	}

	return &InitiatedDepositResponse{
		Status:     true,
		TrackingID: trackingID,
		ExpiresAt:  expiresAt,
		Account:    *account,
	}, nil

}

func (s *Service) HandleCreditWebhook(ctx context.Context, payload *ProvidusCredit) error {
	if strings.TrimSpace(payload.TranType) != "C" {
		return fmt.Errorf("unexpected tranType: %s", payload.TranType)
	}

	amountFloat, err := strconv.ParseFloat(strings.TrimSpace(payload.TransactionAmount), 64)
	if err != nil || amountFloat <= 0 {
		return fmt.Errorf("invalid transaction amount: %s", payload.TransactionAmount)
	}
	amountKobo := int64(math.Round(amountFloat * 100))

	providerRef := strings.TrimSpace(payload.TranID)
	if providerRef == "" {
		providerRef = strings.TrimSpace(payload.SessionID)
	}
	if providerRef == "" {
		return errors.New("no usable provider reference in payload")
	}

	wallet, err := s.repo.GetWalletByAccountNumber(ctx, strings.TrimSpace(payload.AccountNumber))
	if err != nil {
		// account not ours — log upstream and return nil so handler responds 200
		return nil
	}

	// existing, _ := s.repo.GetTransferByProviderRef(ctx, providerRef)
	// if existing != nil {
	// 	return nil // duplicate webhook, already processed
	// }

	narration := strings.TrimSpace(payload.TranRemarks)
	transfer := &transaction.Transaction{
		ID:                  uuid.NewString(),
		MobileUserID:        wallet.MobileUserID,
		WalletID:            wallet.WalletID,
		Reference:           uuid.NewString(),
		ProviderReference:   providerRef,
		SessionID:           payload.SessionID,
		Amount:              amountKobo,
		Narration:           &narration,
		CounterpartyAccount: payload.AccountNumber,
		Description:         payload.TranRemarks,
		Status:              transaction.TransactionStatusSuccessful,
		Type:                transaction.TransactionTypeCredit,
	}

	if err := s.repo.CreditWalletAtomically(ctx, transfer, amountKobo); err != nil {
		return fmt.Errorf("failed to credit wallet: %w", err)
	}

	return nil
}

func (s *Service) GetBeneficiaries(ctx context.Context, mobileUserID string) ([]Beneficiary, error) {
	beneficiaries, err := s.repo.GetBeneficiaries(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrFetchingBeneficiaries
	}

	return beneficiaries, nil
}

func (s *Service) GetUserWalletBalance(ctx context.Context, mobileUserID string) (*CustomerWallet, error) {
	wallet, err := s.repo.GetWallet(ctx, mobileUserID)
	if err != nil {
		log.Printf("wallet service: failed to get user wallet - %s", err)
		return nil, err
	}
	return wallet, nil
}
