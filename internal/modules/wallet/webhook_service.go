package wallet

import (
	"context"
	"errors"
	"log"
	"neat_mobile_app_backend/internal/modules/transaction"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) ProcessCustomerBankTransfer(ctx context.Context, data *CustomerBankTransferData) error {
	tx, err := s.repo.FindTransactionByProviderRef(ctx, data.TransactionReference)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("baas: no pending tx for provider_ref=%s", data.TransactionReference)
			return nil
		}
		return err
	}

	if tx.Status != transaction.TransactionStatusPending {
		log.Printf("baas: tx %s already %s - skipping", tx.ID, tx.Status)
		return nil
	}

	if data.Status == "success" {
		log.Printf("baas: confirming transfer tx=%s ref=%s", tx.ID, data.TransactionReference)
		return s.repo.UpdateTransactionStatus(ctx, tx.ID, transaction.TransactionStatusSuccessful)
	}

	user, err := s.repo.FindUserByWalletCustomerID(ctx, data.CustomerID)
	if err != nil {
		return err
	}

	

	if err := s.SmsSender.Send(ctx, user.Phone, "Your transaction has been successfully processed."); err != nil {
		return err
	}

	log.Printf("baas: reversing failed transfer tx=%s ref=%s", tx.ID, data.TransactionReference)
	return s.repo.ReverseDebitTransaction(ctx, tx.ID, tx.WalletID)
}

func (s *Service) ProcessAccountFunded(ctx context.Context, data *AccountFundedData) error {
	if data.Status != "success" {
		log.Printf("baas: account funded failed status=%s - skipping", data.Status)
		return nil
	}

	wallet, err := s.repo.GetWalletByAccountNumber(ctx, data.AccountNumber)
	if err != nil {
		log.Printf("baas: no wallet found for account_number=%s: %v", data.AccountNumber, err)
		return nil
	}

	existing, err := s.repo.FindTransactionByProviderRef(ctx, data.Reference)
	if err == nil && existing != nil {
		log.Printf("baas: deposit ref=%s already processed as tx=%s", data.Reference, existing.ID)
		return nil
	}

	amountKobo, err := parseAmountToKobo(data.Amount)
	if err != nil {
		log.Printf("baas: failed to parse amount=%q: %v", data.Amount, err)
		return nil
	}

	now := time.Now().UTC()
	creditTx := &transaction.Transaction{
		ID:                uuid.NewString(),
		MobileUserID:      wallet.MobileUserID,
		WalletID:          wallet.WalletID,
		Type:              transaction.TransactionTypeCredit,
		Category:          transaction.TransactionCategoryTransferFrom,
		Source:            transaction.TransactionSourceCredit,
		Amount:            amountKobo,
		Reference:         uuid.NewString(),
		ProviderReference: data.Reference,
		SessionID:         data.SessionID,
		Status:            transaction.TransactionStatusSuccessful,
		Description:       "Deposit via bank transfer",
		CreatedAt:         now,
	}
	return s.repo.CreditWalletAtomically(ctx, creditTx, amountKobo)
}
