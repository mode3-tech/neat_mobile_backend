package vas

import (
	"context"
	"errors"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/phone"
	vasprovider "neat_mobile_app_backend/providers/vas"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	Repo           *Repository
	WalletService  WalletService
	Txr            TransactionService
	Baas           BAAS
	XpressPayments VASService
}

func NewService(repo *Repository, xpressPayments VASService, walletService WalletService, txr TransactionService, baas BAAS) *Service {
	return &Service{Repo: repo, XpressPayments: xpressPayments, WalletService: walletService, Txr: txr, Baas: baas}
}

func (s *Service) GetAirtime(ctx context.Context, payload AirtimePayload, mobileUserID string) (*vasprovider.ISPResponse, error) {
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	localizedPhone, err := phone.ToLocalFormat(strings.TrimSpace(payload.PhoneNumber))
	if err != nil {
		log.Printf("vas service: failed to normalize phone number - %s\n", err)
		return nil, err
	}
	amount := payload.Amount

	if amount < 50 {
		log.Println("vas service: amount is less than NGN 50")
		return nil, appErr.ErrInvalidISPAmount
	}

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get wallet balance - %s\n", err)
		return nil, appErr.ErrGettingAirtime
	}

	if wallet.AvailableBalance < amount*100 {
		log.Println("vas service: insufficient balance")
		return nil, appErr.ErrInsufficientBalance
	}

	metadata := map[string]any{
		"isp":  ExtractBillingCompanyName(uniqueCode),
		"type": "airtime",
	}

	txID, ref := uuid.NewString(), uuid.NewString()

	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryAirtime,
		Amount:              amount * 100,
		BalanceBefore:       wallet.AvailableBalance,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: localizedPhone,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		CreatedAt:           time.Now().UTC(),
	}

	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		log.Printf("vas service: failed to add transaction record at pending state - %s\n", err)
		return nil, err
	}

	debitResult, err := s.Baas.DebitCustomer(ctx, amount, wallet.WalletCustomerID, ref, metadata)
	if err != nil {
		log.Printf("vas service: failed to debit customer wallet - %s\n", err)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, wallet.AvailableBalance, TransactionStatusFailed); updateErr != nil {
			log.Printf("vas service: failed to update transaction to failed after debit error - %s\n", updateErr)
		}
		return nil, appErr.ErrGettingAirtime
	}

	result, err := s.XpressPayments.GetAirtime(ctx, requestID, uniqueCode, localizedPhone, amount)
	if err != nil {
		log.Printf("vas service: unable to purchase airtime - %s\n", err)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, err)
		return nil, appErr.ErrGettingAirtime
	}

	balanceAfter := wallet.AvailableBalance - ((amount + int64(debitResult.Data.TransactionFee)) * 100)
	if err := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); err != nil {
		log.Printf("vas service: failed to update transaction record to successful - %s", err)
		return nil, appErr.ErrGettingAirtime
	}

	return result, nil
}

func (s *Service) GetData(ctx context.Context, payload DataPayload, mobileUserID string) (*vasprovider.ISPResponse, error) {
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	localizedPhone, err := phone.ToLocalFormat(strings.TrimSpace(payload.PhoneNumber))
	if err != nil {
		log.Printf("vas service: failed to normalize phone number - %s\n", err)
		return nil, err
	}
	amount := payload.Amount

	if amount < 50 {
		return nil, appErr.ErrInvalidISPAmount
	}

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get wallet balance - %s\n", err)
		return nil, appErr.ErrGettingData
	}

	if wallet.AvailableBalance < amount*100 {
		return nil, appErr.ErrInsufficientBalance
	}

	metadata := map[string]any{
		"isp":  ExtractBillingCompanyName(uniqueCode),
		"type": "data",
	}

	txID, ref := uuid.NewString(), uuid.NewString()

	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryMobileData,
		Amount:              amount * 100,
		BalanceBefore:       wallet.AvailableBalance,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: localizedPhone,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		CreatedAt:           time.Now().UTC(),
	}

	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		log.Printf("vas service: failed to add transaction record at pending state - %s\n", err)
		return nil, err
	}

	debitResult, err := s.Baas.DebitCustomer(ctx, amount, wallet.WalletCustomerID, ref, metadata)
	if err != nil {
		log.Printf("vas service: failed to debit customer wallet - %s\n", err)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, wallet.AvailableBalance, TransactionStatusFailed); updateErr != nil {
			log.Printf("vas service: failed to update transaction to failed after debit error - %s\n", updateErr)
		}
		return nil, appErr.ErrGettingData
	}

	result, err := s.XpressPayments.GetData(ctx, requestID, uniqueCode, localizedPhone, amount)
	if err != nil {
		log.Printf("vas service: unable to purchase data - %s\n", err)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, err)
		return nil, appErr.ErrGettingData
	}

	balanceAfter := wallet.AvailableBalance - ((amount + int64(debitResult.Data.TransactionFee)) * 100)
	if err := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); err != nil {
		log.Printf("vas service: failed to update transaction record to successful - %s", err)
		return nil, appErr.ErrGettingData
	}

	return result, nil
}

func (s *Service) ValidateElectricity(ctx context.Context, payload ElectricityValidationPayload, mobileUserID string) (*vasprovider.ElectricityValidationResponse, error) {
	result, err := s.XpressPayments.ValidateElectricity(
		ctx,
		uuid.NewString(),
		strings.TrimSpace(payload.UniqueCode),
		strings.TrimSpace(payload.AccountNumber),
		vasprovider.AccountType(payload.AccountType),
	)
	if err != nil {
		log.Printf("vas service: failed to validate electricity account - %s\n", err)
		return nil, appErr.ErrValidatingElectricity
	}
	return result, nil
}

func (s *Service) PayElectricity(ctx context.Context, payload PayElectricityPayload, mobileUserID string) (*vasprovider.PayElectricityResponse, error) {
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	accountNumber := strings.TrimSpace(payload.AccountNumber)
	amount := payload.Amount

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get wallet balance - %s\n", err)
		return nil, appErr.ErrPayingElectricityBill
	}

	if wallet.AvailableBalance < amount*100 {
		return nil, appErr.ErrInsufficientBalance
	}

	metadata := map[string]any{
		"provider": ExtractBillingCompanyName(uniqueCode),
		"type":     "electricity",
	}

	txID, ref := uuid.NewString(), uuid.NewString()

	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryElectricity,
		Amount:              amount * 100,
		BalanceBefore:       wallet.AvailableBalance,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: accountNumber,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		CreatedAt:           time.Now().UTC(),
	}

	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		log.Printf("vas service: failed to add transaction record at pending state - %s\n", err)
		return nil, err
	}

	debitResult, err := s.Baas.DebitCustomer(ctx, amount, wallet.WalletCustomerID, ref, metadata)
	if err != nil {
		log.Printf("vas service: failed to debit customer wallet - %s\n", err)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, wallet.AvailableBalance, TransactionStatusFailed); updateErr != nil {
			log.Printf("vas service: failed to update transaction to failed after debit error - %s\n", updateErr)
		}
		return nil, appErr.ErrPayingElectricityBill
	}

	result, err := s.XpressPayments.PayElectricityBill(
		ctx, requestID, uniqueCode, accountNumber,
		payload.Name, payload.Address, payload.PhoneNumber,
		vasprovider.AccountType(payload.AccountType), amount,
	)
	if err != nil {
		log.Printf("vas service: failed to pay electricity bill - %s\n", err)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, err)
		return nil, appErr.ErrPayingElectricityBill
	}

	balanceAfter := wallet.AvailableBalance - ((amount + int64(debitResult.Data.TransactionFee)) * 100)
	if err := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); err != nil {
		log.Printf("vas service: failed to update transaction record to successful - %s", err)
		return nil, appErr.ErrPayingElectricityBill
	}

	if result.Data.Token != "" {
		tokenMetadata := map[string]any{
			"provider": ExtractBillingCompanyName(uniqueCode),
			"type":     "electricity",
			"token":    result.Data.Token,
			"units":    result.Data.Unit,
		}
		if updateErr := s.Repo.UpdateTransactionMetadata(ctx, txID, tokenMetadata); updateErr != nil {
			log.Printf("vas service: failed to store electricity token in metadata - %s\n", updateErr)
		}
	}

	return result, nil
}

func (s *Service) ValidateCable(ctx context.Context, payload ValidateCablePayload, mobileUserID string) (*vasprovider.CableValidationResponse, error) {
	result, err := s.XpressPayments.ValidateCable(
		ctx,
		uuid.NewString(),
		strings.TrimSpace(payload.UniqueCode),
		strings.TrimSpace(payload.AccountNumber),
		payload.NoOfMonth,
	)
	if err != nil {
		log.Printf("vas service: failed to validate cable account - %s\n", err)
		return nil, appErr.ErrValidatingCable
	}
	return result, nil
}

func (s *Service) PayCable(ctx context.Context, payload PayCablePayload, mobileUserID string) (*vasprovider.PayCableResponse, error) {
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	accountNumber := strings.TrimSpace(payload.AccountNumber)
	amount := payload.Amount

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get wallet balance - %s\n", err)
		return nil, appErr.ErrPayingCableBill
	}

	if wallet.AvailableBalance < amount*100 {
		return nil, appErr.ErrInsufficientBalance
	}

	metadata := map[string]any{
		"provider": ExtractBillingCompanyName(uniqueCode),
		"type":     "cable",
	}

	txID, ref := uuid.NewString(), uuid.NewString()

	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryTV,
		Amount:              amount * 100,
		BalanceBefore:       wallet.AvailableBalance,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: accountNumber,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		CreatedAt:           time.Now().UTC(),
	}

	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		log.Printf("vas service: failed to add transaction record at pending state - %s\n", err)
		return nil, err
	}

	debitResult, err := s.Baas.DebitCustomer(ctx, amount, wallet.WalletCustomerID, ref, metadata)
	if err != nil {
		log.Printf("vas service: failed to debit customer wallet - %s\n", err)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, wallet.AvailableBalance, TransactionStatusFailed); updateErr != nil {
			log.Printf("vas service: failed to update transaction to failed after debit error - %s\n", updateErr)
		}
		return nil, appErr.ErrPayingCableBill
	}

	result, err := s.XpressPayments.PayCableBill(
		ctx, requestID, uniqueCode, accountNumber,
		payload.AccountType, payload.Name, payload.PhoneNumber,
		payload.NoOfMonth, amount,
	)
	if err != nil {
		log.Printf("vas service: failed to pay cable bill - %s\n", err)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, err)
		return nil, appErr.ErrPayingCableBill
	}

	balanceAfter := wallet.AvailableBalance - ((amount + int64(debitResult.Data.TransactionFee)) * 100)
	if err := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); err != nil {
		log.Printf("vas service: failed to update transaction record to successful - %s", err)
		return nil, appErr.ErrPayingCableBill
	}

	return result, nil
}

// handleFulfilFailure handles the post-debit failure path for all fulfil operations.
// ErrVASAmbiguous (timeout/5xx) → marks reversal_pending for manual reconciliation.
// Any other error → credits the customer back and marks reversed.
func (s *Service) handleFulfilFailure(ctx context.Context, txID string, amount int64, txFee int, balanceBefore int64, metadata map[string]any, customerID string, vasErr error) {
	if errors.Is(vasErr, appErr.ErrVASAmbiguous) {
		debitedBalance := balanceBefore - ((amount + int64(txFee)) * 100)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, debitedBalance, TransactionStatusReversalPending); updateErr != nil {
			log.Printf("vas service: failed to mark transaction as reversal_pending - %s\n", updateErr)
		}
		return
	}

	reversalRef := uuid.NewString()
	if _, creditErr := s.Baas.CreditCustomer(ctx, amount, reversalRef, customerID, metadata); creditErr != nil {
		log.Printf("vas service: failed to credit customer back after VAS failure - %s\n", creditErr)
	}
	if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, balanceBefore, TransactionStatusReversed); updateErr != nil {
		log.Printf("vas service: failed to mark transaction as reversed - %s\n", updateErr)
	}
}
