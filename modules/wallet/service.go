package wallet

import (
	"context"
	"errors"
	"fmt"
	"math"
	"neat_mobile_app_backend/modules/transaction"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Service struct {
	repo            *Repository
	providusService ProvidusService
}

func NewService(repo *Repository, providusService ProvidusService) *Service {
	return &Service{repo: repo, providusService: providusService}
}

func (s *Service) FetchBanks(ctx context.Context, mobileUserID, deviceID string) ([]Bank, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	deviceID = strings.TrimSpace(deviceID)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if deviceID == "" {
		return nil, errors.New("device ID is required")
	}

	_, err := s.repo.GetDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify device: %w", err)
	}

	return s.providusService.FetchBanks(ctx)
}

func (s *Service) FetchBankDetails(ctx context.Context, accountNumber, bankCode, mobileUserID, deviceID string) (*BankDetails, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	deviceID = strings.TrimSpace(deviceID)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if deviceID == "" {
		return nil, errors.New("device ID is required")
	}

	_, err := s.repo.GetDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify device: %w", err)
	}

	return s.providusService.FetchBankDetails(ctx, accountNumber, bankCode)
}

const (
	maxPinAttempts  = 5
	pinLockDuration = 30 * time.Minute
)

func (s *Service) InitiateTransfer(ctx context.Context, mobileUserID, deviceID string, req *TransferRequest) (*TransferResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	deviceID = strings.TrimSpace(deviceID)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if deviceID == "" {
		return nil, errors.New("device ID is required")
	}

	_, err := s.repo.GetDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify device: %w", err)
	}

	user, err := s.repo.GetUserForPinVerification(ctx, mobileUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	if user.TransactionPinLockedUntil != nil && user.TransactionPinLockedUntil.After(time.Now().UTC()) {
		return nil, ErrTransactionPinLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PinHash), []byte(req.TransactionPin)); err != nil {
		newAttempts := user.FailedTransactionPinAttempts + 1
		if newAttempts >= maxPinAttempts {
			_ = s.repo.LockTransactionPin(ctx, mobileUserID, time.Now().UTC().Add(pinLockDuration))
			return nil, ErrTransactionPinLocked
		} else {
			_ = s.repo.IncrementFailedPinAttempts(ctx, mobileUserID)
		}
		return nil, fmt.Errorf("%s, you have %d attempt(s) left", ErrWrongTransactionPin.Error(), maxPinAttempts-newAttempts)
	}

	_ = s.repo.ResetPinAttempts(ctx, mobileUserID)

	if amount := req.Amount; amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}
	accountNumber := strings.TrimSpace(req.AccountNumber)
	accountName := strings.TrimSpace(*req.AccountName)

	if accountNumber == "" {
		return nil, fmt.Errorf("account number is required")
	}

	if accountName == "" {
		return nil, fmt.Errorf("account name is required")
	}
	walletUser, err := s.repo.GetUserWalletID(ctx, mobileUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch wallet: %w", err)
	}

	narration := ""
	if req.Narration != nil {
		narration = *req.Narration
	}

	txID := uuid.NewString()
	txRecord := &transaction.Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            walletUser.WalletID,
		Category:            transaction.TransactionCategoryTransferTo,
		Type:                transaction.TransactionTypeDebit,
		Description:         fmt.Sprintf("Transfer to %s", *req.AccountName),
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
		return nil, fmt.Errorf("failed to create transaction record: %w", err)
	}

	resp, err := s.providusService.InitiateTransfer(ctx, req)
	if err != nil {
		_ = s.repo.UpdateTransactionStatus(ctx, txID, transaction.TransactionStatusFailed)
	}

	totalDebit := req.Amount + int64(math.Round(resp.Transfer.Charges*100)) + int64(math.Round(resp.Transfer.Vat*100))

	if err := s.repo.CompleteDebitTransaction(ctx, txID, resp.Transfer.TransactionReference, transaction.TransactionStatusSuccessful, walletUser.WalletID, totalDebit); err != nil {
		return nil, fmt.Errorf("failed to set a successful transaction record: %w", err)
	}

	return resp, nil

}

func (s *Service) AddBeneficiary(ctx context.Context, mobileUserID, deviceID string, req *AddBeneficiaryRequest) (*Beneficiary, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	deviceID = strings.TrimSpace(deviceID)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if deviceID == "" {
		return nil, errors.New("device ID is required")
	}

	_, err := s.repo.GetDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify device: %w", err)
	}

	user, err := s.repo.GetUserWalletID(ctx, mobileUserID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	walletID := strings.TrimSpace(user.WalletID)
	accountNumber := strings.TrimSpace(req.AccountNumber)
	accountName := strings.TrimSpace(req.AccountName)
	bankCode := strings.TrimSpace(req.BankCode)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if walletID == "" {
		return nil, errors.New("wallet ID is required")
	}

	if accountNumber == "" {
		return nil, errors.New("account number is required")
	}

	if accountName == "" {
		return nil, errors.New("account name is required")
	}

	if bankCode == "" {
		return nil, errors.New("bank code is required")
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
		return nil, err
	}

	return beneficiary, nil
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

	existing, _ := s.repo.GetTransferByProviderRef(ctx, providerRef)
	if existing != nil {
		return nil // duplicate webhook, already processed
	}

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

	if err := s.repo.AddTransaction(ctx, transfer); err != nil {
		return fmt.Errorf("failed to save credit transfer: %w", err)
	}

	if err := s.repo.CreditWalletBalance(ctx, wallet.WalletID, amountKobo); err != nil {
		return fmt.Errorf("failed to update wallet balance: %w", err)
	}

	return nil
}

func (s *Service) GetBeneficiaries(ctx context.Context, mobileUserID, deviceID string) ([]Beneficiary, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	deviceID = strings.TrimSpace(deviceID)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if deviceID == "" {
		return nil, errors.New("device ID is required")
	}

	_, err := s.repo.GetDevice(ctx, mobileUserID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify device: %w", err)
	}

	return s.repo.GetBeneficiaries(ctx, mobileUserID)
}
