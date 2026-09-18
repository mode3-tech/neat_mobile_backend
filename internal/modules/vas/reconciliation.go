package vas

import (
	"context"
	"log"
	"strings"
	"time"

	vasprovider "neat_mobile_app_backend/providers/vas"
)

// ReconciliationConfig tunes the stuck-transaction sweep. See
// DefaultReconciliationConfig for the values used in production.
type ReconciliationConfig struct {
	Limit                      int
	StalePendingAfter          time.Duration
	LeaseWindow                time.Duration
	MaxReversalPendingAttempts int
	MaxRefundPendingAttempts   int
}

func DefaultReconciliationConfig() ReconciliationConfig {
	return ReconciliationConfig{
		Limit:                      50,
		StalePendingAfter:          5 * time.Minute,
		LeaseWindow:                1 * time.Minute,
		MaxReversalPendingAttempts: 20,
		MaxRefundPendingAttempts:   50,
	}
}

// ReconcileStuckTransactions is the cron entrypoint for resolving VAS
// purchases that were debited but never reached a terminal state: crashed
// mid-fulfil (stuck pending), ambiguous provider responses that were never
// resolved (reversal_pending), and refunds that didn't fully complete
// (refund_pending). See the package-level comment on handleFulfilFailure
// for how transactions end up in these states.
func (s *Service) ReconcileStuckTransactions(ctx context.Context, cfg ReconciliationConfig) error {
	txns, err := s.Repo.ClaimStuckTransactions(ctx, cfg.Limit, cfg.StalePendingAfter, cfg.LeaseWindow, cfg.MaxReversalPendingAttempts, cfg.MaxRefundPendingAttempts)
	if err != nil {
		return err
	}

	for i := range txns {
		txn := &txns[i]
		switch txn.Status {
		case TransactionStatusRefundPending:
			s.reconcileRefundPending(ctx, txn)
		case TransactionStatusReversalPending, TransactionStatusPending:
			s.reconcileAmbiguousOutcome(ctx, txn)
		}
	}

	return nil
}

func refundMetadataFor(txn *Transaction) map[string]any {
	return map[string]any{
		"category":       string(txn.Category),
		"reconciliation": true,
	}
}

// reconcileRefundPending retries only whatever refund sub-step (wallet
// credit-back or cashback release) hasn't completed yet — the purchase is
// already known to have failed, so no CheckStatus call is needed.
func (s *Service) reconcileRefundPending(ctx context.Context, txn *Transaction) {
	wallet, err := s.WalletService.GetBalance(ctx, txn.MobileUserID)
	if err != nil {
		log.Printf("vas reconciliation: failed to load wallet for refund_pending txID=%s: %v", txn.ID, err)
		return
	}

	walletOK, cashbackOK := s.attemptRefund(ctx, txn, wallet.WalletCustomerID, refundMetadataFor(txn))
	if !(walletOK && cashbackOK) {
		return
	}
	if err := s.Txr.UpdateTransactionStatus(ctx, txn.ID, txn.BalanceBefore, TransactionStatusReversed); err != nil {
		log.Printf("vas reconciliation: failed to mark refund_pending txn reversed txID=%s: %v", txn.ID, err)
	}
}

// reconcileAmbiguousOutcome resolves a reversal_pending row or a stale
// crash-window pending row by asking the provider what actually happened.
func (s *Service) reconcileAmbiguousOutcome(ctx context.Context, txn *Transaction) {
	if strings.TrimSpace(txn.VASRequestID) == "" {
		// Pre-migration orphan with no persisted request ID — nothing to
		// check status on. Left alone per the confirmed decision.
		return
	}

	status, err := s.XpressPayments.CheckStatus(ctx, txn.VASRequestID)
	if err != nil {
		// Still can't resolve — leave as-is, attempt counter already bumped
		// by the claim step, will be retried on the next sweep.
		return
	}

	switch status.ResponseCode {
	case "00":
		balanceAfter := txn.BalanceBefore - txn.Amount
		if err := s.Txr.UpdateTransactionStatus(ctx, txn.ID, balanceAfter, TransactionStatusSuccessful); err != nil {
			log.Printf("vas reconciliation: failed to mark txn successful txID=%s: %v", txn.ID, err)
			return
		}
		if status.ReferenceID != "" {
			if err := s.Txr.UpdateTransactionProviderReference(ctx, txn.ID, status.ReferenceID); err != nil {
				log.Printf("vas reconciliation: failed to store provider reference txID=%s: %v", txn.ID, err)
			}
		}
		s.backfillElectricityMetadata(ctx, txn, status)
	case "01":
		if txn.Status == TransactionStatusReversalPending {
			return
		}
		balanceAfter := txn.BalanceBefore - txn.Amount
		if err := s.Txr.UpdateTransactionStatus(ctx, txn.ID, balanceAfter, TransactionStatusReversalPending); err != nil {
			log.Printf("vas reconciliation: failed to mark txn reversal_pending txID=%s: %v", txn.ID, err)
		}
	default:
		wallet, err := s.WalletService.GetBalance(ctx, txn.MobileUserID)
		if err != nil {
			log.Printf("vas reconciliation: failed to load wallet for refund txID=%s: %v", txn.ID, err)
			return
		}
		walletOK, cashbackOK := s.attemptRefund(ctx, txn, wallet.WalletCustomerID, refundMetadataFor(txn))
		if walletOK && cashbackOK {
			if err := s.Txr.UpdateTransactionStatus(ctx, txn.ID, txn.BalanceBefore, TransactionStatusReversed); err != nil {
				log.Printf("vas reconciliation: failed to mark txn reversed txID=%s: %v", txn.ID, err)
			}
			return
		}
		balanceAfter := txn.BalanceBefore - txn.Amount
		if err := s.Repo.MarkTransactionRefundPending(ctx, txn.ID, balanceAfter); err != nil {
			log.Printf("vas reconciliation: failed to mark txn refund_pending txID=%s: %v", txn.ID, err)
		}
	}
}

// backfillElectricityMetadata makes a best-effort attempt to recover the
// electricity token/units from a CheckStatus response for a purchase that
// turned out to have succeeded despite the original ambiguous response. The
// provider's CheckStatusResponse.Data shape isn't typed (providers/vas
// CheckStatusResponse.Data is interface{}), so this is deliberately
// tolerant of missing/unexpected shapes and never fails the reconciliation.
func (s *Service) backfillElectricityMetadata(ctx context.Context, txn *Transaction, status *vasprovider.CheckStatusResponse) {
	if txn.Category != TransactionCategoryElectricity {
		return
	}
	data, ok := status.Data.(map[string]interface{})
	if !ok {
		return
	}
	token, ok := data["token"].(string)
	if !ok || strings.TrimSpace(token) == "" {
		return
	}

	metadata := map[string]any{
		"provider": txn.CounterpartyName,
		"type":     "electricity",
		"token":    token,
	}
	if unit, ok := data["unit"]; ok {
		metadata["units"] = unit
	}
	if err := s.Repo.UpdateTransactionMetadata(ctx, txn.ID, metadata); err != nil {
		log.Printf("vas reconciliation: failed to backfill electricity metadata txID=%s: %v", txn.ID, err)
	}
}
