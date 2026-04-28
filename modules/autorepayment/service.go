package autorepayment

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"neat_mobile_app_backend/modules/loanproduct"
	"neat_mobile_app_backend/modules/notification"
	"neat_mobile_app_backend/modules/transaction"
	"neat_mobile_app_backend/modules/wallet"

	"github.com/google/uuid"
)

type Service struct {
	repository          *Repository
	walletRepository    *wallet.Repository
	providusService     wallet.ProvidusService
	repayer             loanproduct.ManualRepayer
	notificationService *notification.Service
	settlementAccount   wallet.SettlementAccount
}

func NewService(
	repository *Repository,
	walletRepository *wallet.Repository,
	providusService wallet.ProvidusService,
	repayer loanproduct.ManualRepayer,
	notificationService *notification.Service,
	settlementAccount wallet.SettlementAccount,
) *Service {
	return &Service{
		repository:          repository,
		walletRepository:    walletRepository,
		providusService:     providusService,
		repayer:             repayer,
		notificationService: notificationService,
		settlementAccount:   settlementAccount,
	}
}

func (s *Service) ProcessDueRepayments(ctx context.Context) error {
	dueRepayments, err := s.repository.GetDueRepayments(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch due repayments: %w", err)
	}

	for _, row := range dueRepayments {
		itemCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		s.processSingle(itemCtx, row)
	}

	return nil
}

func (s *Service) processSingle(ctx context.Context, row DueRepaymentRow) {
	exists, err := s.repository.HasActiveAttempt(ctx, row.RepaymentID)
	if err != nil {
		log.Printf("auto-repayment: failed to check existing attempt for repayment %d: %v", row.RepaymentID, err)
		return
	}
	if exists {
		return
	}

	attemptID := uuid.NewString()
	if err := s.repository.InsertAttempt(ctx, &AutoRepaymentAttempt{
		ID:              attemptID,
		LoanRepaymentID: row.RepaymentID,
		MobileUserID:    row.MobileUserID,
		Amount:          row.Amount,
		Status:          AutoRepaymentAttemptStatusPending,
		AttemptedAt:     time.Now(),
	}); err != nil {
		log.Printf("auto-repayment: failed to insert attempt for repayment %d: %v", row.RepaymentID, err)
		return
	}

	walletUser, err := s.walletRepository.GetUserWalletID(ctx, row.MobileUserID)
	if err != nil {
		log.Printf("auto-repayment: failed to fetch wallet user for %s: %v", row.MobileUserID, err)
		_ = s.repository.UpdateAttemptStatus(ctx, attemptID, AutoRepaymentAttemptStatusFailed, err.Error(), "")
		return
	}

	w, err := s.walletRepository.GetWallet(ctx, row.MobileUserID, walletUser.WalletID)
	if err != nil {
		log.Printf("auto-repayment: failed to fetch wallet for %s: %v", row.MobileUserID, err)
		_ = s.repository.UpdateAttemptStatus(ctx, attemptID, AutoRepaymentAttemptStatusFailed, err.Error(), "")
		return
	}

	amountKobo := row.Amount * 100
	if w.AvailableBalance < amountKobo {
		_ = s.repository.UpdateAttemptStatus(ctx, attemptID, AutoRepaymentAttemptStatusSkipped, "insufficient balance", "")
		_ = s.notificationService.SendToUser(ctx, row.MobileUserID,
			"Auto-repayment skipped", "loan",
			"Your loan auto-repayment was skipped due to insufficient wallet balance. Please top up to avoid penalties.",
			nil)
		return
	}

	narration := "Loan auto-repayment"
	accountName := s.settlementAccount.AccountName
	txID := uuid.NewString()
	if err := s.walletRepository.AddTransaction(ctx, &transaction.Transaction{
		ID:                  txID,
		MobileUserID:        row.MobileUserID,
		WalletID:            walletUser.WalletID,
		Type:                transaction.TransactionTypeDebit,
		Category:            transaction.TransactionCategoryLoanRepayment,
		Source:              transaction.TransactionSourceAutoRepayment,
		Amount:              amountKobo,
		Reference:           uuid.NewString(),
		Narration:           &narration,
		CounterpartyAccount: s.settlementAccount.AccountNumber,
		CounterpartyName:    accountName,
		CounterpartyBank:    s.settlementAccount.BankCode,
		Status:              transaction.TransactionStatusPending,
	}); err != nil {
		log.Printf("auto-repayment: failed to create transaction for repayment %d: %v", row.RepaymentID, err)
		_ = s.repository.UpdateAttemptStatus(ctx, attemptID, AutoRepaymentAttemptStatusFailed, err.Error(), "")
		return
	}

	resp, err := s.providusService.InitiateTransfer(ctx, w.WalletCustomerID, &wallet.TransferRequest{
		Amount:        row.Amount,
		SortCode:      s.settlementAccount.BankCode,
		AccountNumber: s.settlementAccount.AccountNumber,
		AccountName:   &accountName,
		Narration:     &narration,
	})
	if err != nil || resp == nil || !resp.Status {
		reason := "provider transfer failed"
		if err != nil {
			reason = err.Error()
		} else if strings.TrimSpace(resp.Message) != "" {
			reason = resp.Message
		}
		_ = s.walletRepository.UpdateTransactionStatus(ctx, txID, transaction.TransactionStatusFailed)
		_ = s.repository.UpdateAttemptStatus(ctx, attemptID, AutoRepaymentAttemptStatusFailed, reason, "")
		_ = s.notificationService.SendToUser(ctx, row.MobileUserID,
			"Auto-repayment failed", "loan",
			"Your loan auto-repayment could not be processed. Please repay manually.", nil)
		return
	}

	totalDebit := amountKobo + int64(math.Round(resp.Transfer.Charges*100)) + int64(math.Round(resp.Transfer.Vat*100))
	if err := s.walletRepository.CompleteDebitTransaction(ctx, txID, resp.Transfer.TransactionReference,
		transaction.TransactionStatusSuccessful, walletUser.WalletID, totalDebit); err != nil {
		log.Printf("auto-repayment: failed to complete debit for repayment %d: %v", row.RepaymentID, err)
		_ = s.repository.UpdateAttemptStatus(ctx, attemptID, AutoRepaymentAttemptStatusFailed, err.Error(), resp.Transfer.TransactionReference)
		return
	}

	_, err = s.repayer.MakeManualRepayment(ctx, loanproduct.RepaymentRequest{
		Amount:      row.Amount,
		RepaymentID: strconv.FormatInt(row.LoanID, 10),
	})
	if err != nil {
		// Wallet already debited — provider_ref recorded for ops reconciliation
		log.Printf("auto-repayment: CBA confirmation failed for repayment %d (wallet debited, provider_ref=%s): %v",
			row.RepaymentID, resp.Transfer.TransactionReference, err)
		_ = s.repository.UpdateAttemptStatus(ctx, attemptID, AutoRepaymentAttemptStatusFailed, err.Error(), resp.Transfer.TransactionReference)
		_ = s.notificationService.SendToUser(ctx, row.MobileUserID,
			"Auto-repayment pending confirmation", "loan",
			"Your auto-repayment was processed but core banking confirmation is pending. Contact support if your loan balance does not update.",
			nil)
		return
	}

	_ = s.repository.UpdateAttemptStatus(ctx, attemptID, AutoRepaymentAttemptStatusSuccess, "", resp.Transfer.TransactionReference)
	_ = s.notificationService.SendToUser(ctx, row.MobileUserID,
		"Auto-repayment successful", "loan",
		fmt.Sprintf("Your loan auto-repayment of ₦%d was successful.", row.Amount),
		nil)
}
