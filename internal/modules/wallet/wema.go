package wallet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"
	"neat_mobile_app_backend/internal/modules/transaction"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Wallet rows record whichever name WALLET_PROVIDER held when the account was
// created, so a Wema ALAT wallet may be stored as either.
const (
	ProviderWema = "wema"
	ProviderAlat = "alat"
)

// IsWemaProvider reports whether a wallet's Provider value means Wema ALAT.
func IsWemaProvider(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	return p == ProviderWema || p == ProviderAlat
}

// Token signer purposes - see internal/adapters/wema/security.go.
const (
	SignPurposeSecurityInfo = "alat-security-info"
	SignPurposeStatement    = "alat-statement"
)

// TokenSigner signs and verifies values with a secret only we hold.
type TokenSigner interface {
	Sign(purpose, value string) (string, error)
	Verify(purpose, value, signature string) bool
}

// ProviderTransferStatus is a provider's final (or not yet final) view of a
// transfer, normalised across providers.
type ProviderTransferStatus string

const (
	ProviderTransferPending    ProviderTransferStatus = "pending"
	ProviderTransferSuccessful ProviderTransferStatus = "successful"
	ProviderTransferFailed     ProviderTransferStatus = "failed"
)

// ProviderTransaction is one entry of a provider-side transaction history or
// statement.
type ProviderTransaction struct {
	Title                    string  `json:"title"`
	Amount                   float64 `json:"amount"`
	Type                     string  `json:"type"`
	Date                     string  `json:"date"`
	TransactionDate          string  `json:"transaction_date,omitempty"`
	Narration                string  `json:"narration"`
	Status                   string  `json:"status"`
	CreditType               string  `json:"credit_type"`
	Sender                   string  `json:"sender"`
	SenderAccountNumber      string  `json:"sender_account_number"`
	DestinationBank          string  `json:"destination_bank"`
	DestinationAccountNumber string  `json:"destination_account_number"`
	ReceiverName             string  `json:"receiver_name"`
	ReferenceID              string  `json:"reference_id"`
	IsViewReceiptEnabled     bool    `json:"is_view_receipt_enabled"`
	TranID                   string  `json:"tran_id"`
}

// WemaProvider is the Wema ALAT functionality that doesn't fit
// TransferProviderService: polled transfer status, payouts into a wallet, and
// ALAT-side history and statements.
type WemaProvider interface {
	CheckDebitStatus(ctx context.Context, reference string) (ProviderTransferStatus, error)
	CheckCreditStatus(ctx context.Context, reference string) (ProviderTransferStatus, error)
	FundWallet(ctx context.Context, accountNumber string, amountNaira float64, narration, reference string) (*TransferResponse, error)
	TransactionHistory(ctx context.Context, accountNumber string, from, to time.Time, keyword string) ([]ProviderTransaction, error)
	InitiateStatement(ctx context.Context, accountNumber string, from, to time.Time) (string, error)
	StatementTransactions(ctx context.Context, referenceID string) ([]ProviderTransaction, error)
}

// ConfigureWema enables the Wema-only features. Without it they return
// ErrWemaUnavailable, and the reconciliation sweep is a no-op.
func (s *Service) ConfigureWema(provider WemaProvider, signer TokenSigner) {
	s.wema = provider
	s.signer = signer
}

// customerLookupKey is what to pass to TransferProviderService.GetCustomerDetails
// for w: ALAT looks wallets up by account number, the others by customer ID.
func customerLookupKey(w *CustomerWallet) string {
	if IsWemaProvider(w.Provider) {
		return w.AccountNumber
	}
	return w.WalletCustomerID
}

// providerForUser returns the transfer provider that issued the user's
// wallet, or the default provider when the user has no wallet yet.
func (s *Service) providerForUser(ctx context.Context, mobileUserID string) TransferProviderService {
	if strings.TrimSpace(mobileUserID) == "" {
		return s.defaultTransferProvider()
	}
	w, err := s.repo.GetWallet(ctx, mobileUserID)
	if err != nil {
		return s.defaultTransferProvider()
	}
	p, err := s.transferProviderForWallet(w)
	if err != nil {
		return s.defaultTransferProvider()
	}
	return p
}

// wemaWallet returns the user's wallet, requiring it to be a Wema ALAT one.
func (s *Service) wemaWallet(ctx context.Context, mobileUserID string) (*CustomerWallet, error) {
	if s.wema == nil {
		return nil, appErr.ErrWemaUnavailable
	}
	w, err := s.repo.GetWallet(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrMissingUserWallet
		}
		return nil, err
	}
	if !IsWemaProvider(w.Provider) {
		return nil, appErr.ErrWemaWalletRequired
	}
	return w, nil
}

// WemaAccountNumber returns the user's Wema account number; ok is false when
// the user's wallet isn't a Wema one.
func (s *Service) WemaAccountNumber(ctx context.Context, mobileUserID string) (string, bool, error) {
	w, err := s.repo.GetWallet(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if !IsWemaProvider(w.Provider) {
		return "", false, nil
	}
	return w.AccountNumber, true, nil
}

// AuthorizeAlatTransaction answers ALAT's Authentication Callback. ALAT asks
// us to confirm a debit/payout by echoing back the transactionReference and
// securityInfo we sent. It is approved only if securityInfo is our signature
// of that reference and the reference is one of our still-pending
// transactions (the Wema adapter uses our transaction ID as the reference).
func (s *Service) AuthorizeAlatTransaction(ctx context.Context, reference, securityInfo string) bool {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		log.Printf("wallet service: alat auth callback rejected: empty reference")
		return false
	}
	tx, err := s.repo.FindTransactionByID(ctx, reference)
	if err != nil {
		log.Printf("wallet service: alat auth callback rejected ref=%s: transaction lookup: %v", reference, err)
		return false
	}
	if tx.Status != transaction.TransactionStatusPending {
		log.Printf("wallet service: alat auth callback rejected ref=%s: transaction is %s", reference, tx.Status)
		return false
	}
	log.Printf("wallet service: alat auth callback approved ref=%s type=%s amount_kobo=%d", reference, tx.Type, tx.Amount)
	return true
}

// looksLikeDashlessUUID reports whether s is 32 hex characters - one of our
// transaction IDs (UUIDs) with the dashes removed.
func looksLikeDashlessUUID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

// Pending Wema transfers younger than wemaReconcileMinAge are left alone so
// ALAT has time to process them; older than wemaReconcileMaxAge they're
// dropped from the sweep (still pending - investigate by hand).
const (
	wemaReconcileMinAge = 30 * time.Second
	wemaReconcileMaxAge = 7 * 24 * time.Hour
)

// ReconcilePendingWemaTransactions polls ALAT for the final status of pending
// Wema debits and payouts and settles them in the local ledger: debits are
// marked successful or reversed; payouts are credited or marked failed. It is
// the fallback for HandleAlatTransactionCallback: it settles transactions
// whose callback never came or came before ALAT reported a final status.
func (s *Service) ReconcilePendingWemaTransactions(ctx context.Context, limit int) error {
	if s.wema == nil {
		return nil
	}
	now := time.Now().UTC()
	txs, err := s.repo.FindPendingProviderTransactions(ctx, []string{ProviderWema, ProviderAlat}, now.Add(-wemaReconcileMinAge), now.Add(-wemaReconcileMaxAge), limit)
	if err != nil {
		return fmt.Errorf("find pending wema transactions: %w", err)
	}
	for i := range txs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.reconcileWemaTransaction(ctx, &txs[i])
	}
	return nil
}

func (s *Service) reconcileWemaTransaction(ctx context.Context, tx *transaction.Transaction) {
	ref := tx.ProviderReference
	isCredit := tx.Type == transaction.TransactionTypeCredit

	var (
		status ProviderTransferStatus
		err    error
	)
	if isCredit {
		status, err = s.wema.CheckCreditStatus(ctx, ref)
	} else {
		status, err = s.wema.CheckDebitStatus(ctx, ref)
	}
	if err != nil {
		log.Printf("wallet service: wema reconcile tx=%s ref=%s status check failed: %v", tx.ID, ref, err)
		s.markReconcileAttempt(ctx, tx.ID)
		return
	}

	switch status {
	case ProviderTransferSuccessful:
		if isCredit {
			err = s.repo.CompleteCreditTransaction(ctx, tx.ID, tx.WalletID)
		} else {
			err = s.repo.MarkPendingTransactionStatus(ctx, tx.ID, transaction.TransactionStatusSuccessful)
		}
	case ProviderTransferFailed:
		if isCredit {
			err = s.repo.MarkPendingTransactionStatus(ctx, tx.ID, transaction.TransactionStatusFailed)
		} else {
			err = s.repo.ReverseDebitTransaction(ctx, tx.ID, tx.WalletID)
		}
	default:
		s.markReconcileAttempt(ctx, tx.ID)
		return
	}
	if err != nil {
		log.Printf("wallet service: wema reconcile tx=%s ref=%s failed to settle as %s: %v", tx.ID, ref, status, err)
		s.markReconcileAttempt(ctx, tx.ID)
		return
	}
	log.Printf("wallet service: wema reconcile tx=%s ref=%s type=%s settled as %s", tx.ID, ref, tx.Type, status)
	s.logWalletAudit(ctx, auditlog.ResourceTypeTransaction, "WEMA_TRANSFER_RECONCILED", tx.MobileUserID, tx.ID, auditlog.StatusSuccess, map[string]interface{}{
		"type":   string(tx.Type),
		"status": string(status),
	})
}

// HandleAlatTransactionCallback settles one Wema debit or payout when ALAT's
// Transaction Callback reports on it. The callback is unsigned, so its status
// is never trusted: the transaction is settled through the same ALAT status
// check the reconciliation sweep uses. Unknown, non-Wema and already-settled
// references are ignored; anything ALAT doesn't yet report as final is left
// pending for the sweep.
func (s *Service) HandleAlatTransactionCallback(ctx context.Context, reference string) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		log.Printf("wallet service: alat transaction callback ignored: empty transactionReference")
		return
	}
	if s.wema == nil {
		log.Printf("wallet service: alat transaction callback ref=%s ignored: wema not configured", reference)
		return
	}
	tx, err := s.repo.FindPendingProviderTransactionByID(ctx, reference, []string{ProviderWema, ProviderAlat})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("wallet service: alat transaction callback ref=%s ignored: not a pending wema transaction", reference)
		return
	}
	if err != nil {
		log.Printf("wallet service: alat transaction callback ref=%s: transaction lookup failed, leaving it to the sweep: %v", reference, err)
		return
	}
	s.reconcileWemaTransaction(ctx, tx)
}

func (s *Service) markReconcileAttempt(ctx context.Context, txID string) {
	if err := s.repo.MarkReconciliationAttempt(ctx, txID); err != nil {
		log.Printf("wallet service: failed to record reconcile attempt tx=%s: %v", txID, err)
	}
}

// FundWalletRequest is an internal (back-office) request to pay money from
// our ALAT payout account into a user's Wema wallet.
type FundWalletRequest struct {
	MobileUserID string  `json:"mobile_user_id" binding:"required"`
	Amount       float64 `json:"amount" binding:"required,gt=0"` // naira
	Narration    string  `json:"narration" binding:"omitempty,max=255"`
	// Reference is the caller's idempotency key: a retry with the same
	// reference returns the original transaction instead of paying twice.
	Reference string `json:"reference" binding:"required,max=128"`
}

// FundWalletResult reports the local transaction created for a payout.
type FundWalletResult struct {
	TransactionID string                        `json:"transaction_id"`
	Reference     string                        `json:"reference"`
	Status        transaction.TransactionStatus `json:"status"`
	Duplicate     bool                          `json:"duplicate"`
}

// FundWemaWallet pays req.Amount from the ALAT payout account into the user's
// Wema wallet (Credit Wallet FundWallet). The local credit is applied only
// once the reconciliation sweep sees ALAT report it successful.
func (s *Service) FundWemaWallet(ctx context.Context, req *FundWalletRequest) (*FundWalletResult, error) {
	if req == nil || req.Amount <= 0 || strings.TrimSpace(req.Reference) == "" {
		return nil, appErr.ErrInvalidRequestBody
	}
	reference := strings.TrimSpace(req.Reference)

	if existing, err := s.repo.FindTransactionByReference(ctx, reference); err == nil {
		return &FundWalletResult{TransactionID: existing.ID, Reference: reference, Status: existing.Status, Duplicate: true}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("look up payout reference: %w", err)
	}

	w, err := s.wemaWallet(ctx, req.MobileUserID)
	if err != nil {
		return nil, err
	}

	amountKobo := int64(math.Round(req.Amount * 100))
	narration := strings.TrimSpace(req.Narration)
	if narration == "" {
		narration = "Wallet funding"
	}
	txID := uuid.NewString()
	txRecord := &transaction.Transaction{
		ID:                  txID,
		MobileUserID:        w.MobileUserID,
		WalletID:            w.InternalWalletID,
		Type:                transaction.TransactionTypeCredit,
		Category:            transaction.TransactionCategoryTransferFrom,
		Source:              transaction.TransactionSourceCredit,
		Description:         narration,
		Amount:              amountKobo,
		Reference:           reference,
		Narration:           &narration,
		CounterpartyAccount: w.AccountNumber,
		CounterpartyName:    w.AccountName,
		CounterpartyBank:    w.BankName,
		Status:              transaction.TransactionStatusPending,
	}
	if err := s.repo.AddTransaction(ctx, txRecord); err != nil {
		return nil, fmt.Errorf("create payout transaction: %w", err)
	}

	resp, err := s.wema.FundWallet(ctx, w.AccountNumber, float64(amountKobo)/100, narration, txID)
	if err == nil && (resp == nil || !resp.Status) {
		msg := "provider returned an unsuccessful payout response"
		if resp != nil && strings.TrimSpace(resp.Message) != "" {
			msg = resp.Message
		}
		err = errors.New(msg)
	}
	if err != nil {
		if updateErr := s.repo.UpdateTransactionStatus(ctx, txID, transaction.TransactionStatusFailed); updateErr != nil {
			log.Printf("wallet service: failed to mark payout tx=%s failed: %v", txID, updateErr)
		}
		s.logWalletAudit(ctx, auditlog.ResourceTypeTransaction, "WEMA_FUND_WALLET", w.MobileUserID, txID, auditlog.StatusFailure, map[string]interface{}{
			"amount_kobo":        amountKobo,
			"reference":          reference,
			"reason_for_failure": err.Error(),
		})
		return nil, err
	}

	if err := s.repo.UpdateTransactionProviderRef(ctx, txID, txID, transaction.TransactionStatusPending); err != nil {
		return nil, fmt.Errorf("record payout provider reference: %w", err)
	}
	s.logWalletAudit(ctx, auditlog.ResourceTypeTransaction, "WEMA_FUND_WALLET", w.MobileUserID, txID, auditlog.StatusSuccess, map[string]interface{}{
		"amount_kobo": amountKobo,
		"reference":   reference,
	})
	return &FundWalletResult{TransactionID: txID, Reference: reference, Status: transaction.TransactionStatusPending}, nil
}

// GetWemaTransactionHistory returns ALAT's own transaction history for the
// user's Wema wallet.
func (s *Service) GetWemaTransactionHistory(ctx context.Context, mobileUserID string, from, to time.Time, keyword string) ([]ProviderTransaction, error) {
	w, err := s.wemaWallet(ctx, mobileUserID)
	if err != nil {
		return nil, err
	}
	items, err := s.wema.TransactionHistory(ctx, w.AccountNumber, from, to, strings.TrimSpace(keyword))
	if err != nil {
		log.Printf("wallet service: wema transaction history failed user=%s: %v", mobileUserID, err)
		return nil, err
	}
	if items == nil {
		items = []ProviderTransaction{}
	}
	return items, nil
}

// InitiateWemaStatement asks ALAT to prepare a statement and returns an
// opaque statement ID: ALAT's reference ID plus our signature binding it to
// this user, so GetWemaStatement can check ownership without storing it.
func (s *Service) InitiateWemaStatement(ctx context.Context, mobileUserID string, from, to time.Time) (string, error) {
	w, err := s.wemaWallet(ctx, mobileUserID)
	if err != nil {
		return "", err
	}
	if s.signer == nil {
		return "", appErr.ErrWemaUnavailable
	}
	ref, err := s.wema.InitiateStatement(ctx, w.AccountNumber, from, to)
	if err != nil {
		log.Printf("wallet service: wema initiate statement failed user=%s: %v", mobileUserID, err)
		return "", err
	}
	sig, err := s.signer.Sign(SignPurposeStatement, statementSigningValue(mobileUserID, ref))
	if err != nil {
		return "", err
	}
	return ref + "." + sig, nil
}

// GetWemaStatement returns the transactions of a statement started by
// InitiateWemaStatement for the same user.
func (s *Service) GetWemaStatement(ctx context.Context, mobileUserID, statementID string) ([]ProviderTransaction, error) {
	if _, err := s.wemaWallet(ctx, mobileUserID); err != nil {
		return nil, err
	}
	i := strings.LastIndex(statementID, ".")
	if i <= 0 || i == len(statementID)-1 || s.signer == nil {
		return nil, appErr.ErrInvalidStatementID
	}
	ref, sig := statementID[:i], statementID[i+1:]
	if !s.signer.Verify(SignPurposeStatement, statementSigningValue(mobileUserID, ref), sig) {
		return nil, appErr.ErrInvalidStatementID
	}
	items, err := s.wema.StatementTransactions(ctx, ref)
	if err != nil {
		log.Printf("wallet service: wema statement fetch failed user=%s: %v", mobileUserID, err)
		return nil, err
	}
	if items == nil {
		items = []ProviderTransaction{}
	}
	return items, nil
}

func statementSigningValue(mobileUserID, ref string) string {
	return strings.TrimSpace(mobileUserID) + "|" + ref
}
