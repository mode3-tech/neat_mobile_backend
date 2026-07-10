package wallet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/modules/transaction"
	"neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) ProcessCustomerBankTransfer(ctx context.Context, data *CustomerBankTransferData) error {
	log.Println("baas: processing customer bank transfer")
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

			wallet, err := s.repo.GetWallet(context.Background(), user.ID)
			if err != nil {
				log.Printf("baas: failed to get wallet for debit sms: user_id=%s err=%v", user.ID, err)
				return
			}

			customerDetails, err := s.providusService.GetCustomerDetails(context.Background(), wallet.WalletCustomerID)
			if err != nil {
				log.Printf("baas: skipping debit sms, failed to get customer details: customer_id=%s err=%v", wallet.WalletCustomerID, err)
				return
			}
			if customerDetails == nil {
				log.Printf("baas: skipping debit sms, nil customer details: customer_id=%s", wallet.WalletCustomerID)
				return
			}

			acc := maskAccount(customerDetails.Customer.AccountNumber)
			balanceNaira := customerDetails.Customer.AvailableBalance

			desc := strings.TrimSpace(data.Description)
			if desc == "" {
				desc = "Transfer"
			}

			msg := fmt.Sprintf("DEBIT ALERT\nAcc: %s\nAmt: NGN%.2f\nBal: NGN%.2f\nDate: %s\nDes: %s",
				acc, data.Amount, balanceNaira, time.Now().Format("02-Jan-2006 15:04"), desc)

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

// fundedBeneficiaryAccountNumber returns the account number of the wallet to
// credit. Real transfer-in payloads put the recipient (our customer) in
// beneficiaryAccountNumber, while accountNumber is the payer. We fall back to
// accountNumber for the provider's documented single-account sample shape.
func fundedBeneficiaryAccountNumber(data *AccountFundedData) string {
	if acctNo := strings.TrimSpace(data.BeneficiaryAccountNumber); acctNo != "" {
		return acctNo
	}
	return strings.TrimSpace(data.AccountNumber)
}

func (s *Service) ProcessAccountFunded(ctx context.Context, data *AccountFundedData) error {
	log.Println("baas: processing account funded")
	if data.Status != "success" {
		log.Printf("baas: account funded failed status=%s - skipping", data.Status)
		return nil
	}

	acctNo := fundedBeneficiaryAccountNumber(data)

	wallet, err := s.repo.GetWalletByAccountNumber(ctx, acctNo)
	if err != nil {
		log.Printf("baas: no wallet found for account_number=%s: %v", acctNo, err)
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
	// The counterparty on an inbound deposit is the sender (originator).
	var narration *string
	if n := strings.TrimSpace(data.Narration); n != "" {
		narration = &n
	}
	creditTx := &transaction.Transaction{
		ID:                  uuid.NewString(),
		MobileUserID:        wallet.MobileUserID,
		WalletID:            wallet.WalletID,
		Type:                transaction.TransactionTypeCredit,
		Category:            transaction.TransactionCategoryTransferFrom,
		Source:              transaction.TransactionSourceCredit,
		Amount:              amountKobo,
		Reference:           uuid.NewString(),
		ProviderReference:   data.Reference,
		SessionID:           data.SessionID,
		Narration:           narration,
		CounterpartyName:    strings.TrimSpace(data.OriginatorAccountName),
		CounterpartyAccount: strings.TrimSpace(data.OriginatorAccountNumber),
		Status:              transaction.TransactionStatusSuccessful,
		Description:         "Deposit via bank transfer",
		CreatedAt:           now,
	}
	if err := s.repo.CreditWalletAtomically(ctx, creditTx, amountKobo); err != nil {
		log.Printf("baas: failed to credit wallet: %v", err)
		return nil
	}

	go func() {
		customerDetails, err := s.providusService.GetCustomerDetails(ctx, wallet.WalletCustomerID)
		if err != nil {
			log.Printf("baas: failed to get customer details: %v", err)
			return
		}
		msg := fmt.Sprintf("%s: ₦%.2f has been credited to your account. New balance: ₦%.2f. Ref: %s", s.appName, float64(amountKobo)/100, float64(customerDetails.Customer.AvailableBalance), creditTx.Reference)
		normalized, err := phone.NormalizeNigerianNumber(wallet.PhoneNumber)
		if err != nil {
			log.Printf("baas: failed to normalize phone number: %v", err)
			return
		}
		if err := s.SmsSender.Send(context.Background(), normalized, msg); err != nil {
			log.Printf("baas: failed to send credit sms: phone=%s err=%v", normalized, err)
		}
	}()

	if s.Notifier != nil {
		go func() {
			amountNaira := float64(amountKobo) / 100
			title := "Money received"
			body := fmt.Sprintf("₦%.2f has been credited to your account.", amountNaira)
			data := map[string]any{
				"reference": creditTx.Reference,
				"amount":    amountNaira,
				"type":      "credit",
			}
			if err := s.Notifier.SendToUser(context.Background(), wallet.MobileUserID, title, models.NotificationTypeTransaction, body, data); err != nil {
				log.Printf("baas: failed to send credit notification: user=%s err=%v", wallet.MobileUserID, err)
			}
		}()
	}

	return nil
}

func maskAccount(a string) string {
	if n := len(a); n >= 6 {
		return a[:3] + "***" + a[n-3:]
	}
	return a
}
