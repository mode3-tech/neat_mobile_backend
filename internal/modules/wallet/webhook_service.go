package wallet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/modules/transaction"
	"neat_mobile_app_backend/internal/phone"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) ProcessCustomerBankTransfer(ctx context.Context, data *CustomerBankTransferData) error {
	tx, err := s.repo.FindTransactionByProviderRef(ctx, data.TransactionReference)
	if tx != nil && err == nil {
		log.Printf("baas: tx %s already %s - skipping", tx.ID, tx.Status)
		return nil
	}
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

		if err := s.repo.UpdateTransactionStatus(ctx, tx.ID, transaction.TransactionStatusSuccessful); err != nil {
			return err
		}

		go func() {
			user, err := s.repo.FindUserByWalletCustomerID(context.Background(), data.CustomerID)
			if err != nil {
				log.Printf("baas: failed to find user for debit sms: customer_id=%s err=%v", data.CustomerID, err)
				return
			}
			amountNaira := data.Amount // assuming it's in naira from the webhook
			msg := fmt.Sprintf("%s: ₦%.2f has been debited from your account. Ref: %s", s.appName, amountNaira, data.TransactionReference)
			normalized, err := phone.NormalizeNigerianNumber(user.Phone)
			if err != nil {
				log.Printf("baas: failed to normalize phone number: %v", err)
				return
			}
			if err := s.SmsSender.Send(context.Background(), normalized, msg); err != nil {
				log.Printf("baas: failed to send debit sms: phone=%s err=%v", normalized, err)
			}
		}()
		return nil
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
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("baas: failed to check for existing deposit ref=%s: %v", data.Reference, err)
		return err
	}
	if existing != nil {
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
	if err := s.repo.CreditWalletAtomically(ctx, creditTx, amountKobo); err != nil {
		log.Printf("baas: failed to credit wallet: %v", err)
		return nil
	}

	go func() {
		msg := fmt.Sprintf("%s: ₦%.2f has been credited to your account. Ref: %s", s.appName, amountKobo/100, creditTx.Reference)
		normalized, err := phone.NormalizeNigerianNumber(wallet.PhoneNumber)
		if err != nil {
			log.Printf("baas: failed to normalize phone number: %v", err)
			return
		}
		if err := s.SmsSender.Send(context.Background(), normalized, msg); err != nil {
			log.Printf("baas: failed to send credit sms: phone=%s err=%v", normalized, err)
		}
	}()

	return nil
}
