package referrals

import (
	"context"
	"errors"
	"log"
	"neat_mobile_app_backend/internal/modules/transaction"
	"neat_mobile_app_backend/models"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RedeemReferralCode(ctx context.Context, mobileUserID, code string) error {
	referral, err := s.repo.FindReferralByCode(ctx, code)
	if err != nil {
		return err
	}
	if referral == nil {
		return nil
	}

	redeemedReferral := &ReferralRedemption{
		ID:             uuid.NewString(),
		ReferrerUserID: referral.MobileUserID,
		ReferredUserID: mobileUserID,
	}

	if err := s.repo.RedeemReferral(ctx, redeemedReferral); err != nil {
		return err
	}

	if err := s.CreditReferralCashback(ctx, referral.MobileUserID); err != nil {
		log.Printf("referrals: cashback credit failed referrer=%s referred=%s: %v", referral.MobileUserID, mobileUserID, err)
	}

	return nil
}

// CreditReferralCashback pays the referrer ReferralCashbackAmountKobo for a
// successful redemption. Cashback is not withdrawable so it never touches the
// wallet's available balance — it is tracked in wallet_cashbacks with running
// totals (cashback_before -> cashback_after) and mirrored by a credit row in
// wallet_transactions whose balance_before/balance_after snapshot the cashback
// ledger.

func (s *Service) CreditReferralCashback(ctx context.Context, referrerUserID string) error {
	return s.repo.WithTx(ctx, func(r *Repository) error {

		walletID, err := r.GetUserWalletIDForUpdate(ctx, referrerUserID)
		if err != nil {
			return err
		}

		var before int64
		last, err := r.GetLatestCashback(ctx, referrerUserID)
		switch {
		case err == nil:
			before = last.CashbackAfter
		case errors.Is(err, gorm.ErrRecordNotFound):
			before = 0
		default:
			return err
		}
		after := before + ReferralCashbackAmountKobo

		cashback := &models.Cashback{
			ID:             uuid.NewString(),
			MobileUserID:   referrerUserID,
			CashbackBefore: before,
			CashbackAfter:  after,
			Source:         CashbackSourceReferral,
			CreatedAt:      time.Now().UTC(),
		}
		if err := r.CreateCashback(ctx, cashback); err != nil {
			return err
		}

		narration := "Referral cashback"
		txRow := &transaction.Transaction{
			ID:            uuid.NewString(),
			MobileUserID:  referrerUserID,
			WalletID:      walletID,
			Type:          transaction.TransactionTypeCredit,
			Category:      transaction.TransactionCategoryCashback,
			Amount:        ReferralCashbackAmountKobo,
			BalanceBefore: before,
			BalanceAfter:  after,
			Reference:     uuid.NewString(),
			Status:        transaction.TransactionStatusSuccessful,
			Source:        transaction.TransactionSourceReferral,
			Description:   "Referral cashback",
			Narration:     &narration,
			CreatedAt:     time.Now().UTC(),
		}

		return transaction.NewServie(transaction.NewRepository(r.db)).AddTransaction(ctx, txRow)
	})
}

func (s *Service) FetchRedeemReferrals(ctx context.Context, page, pageSize int) ([]RedeemedReferral, error) {
	return s.repo.FetchRedeemReferrals(ctx, page, pageSize)
}
