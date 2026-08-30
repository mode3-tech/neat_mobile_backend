package referrals

import (
	"context"
	"errors"
	"fmt"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/modules/transaction"
	"neat_mobile_app_backend/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const (
	referralCodeLength     = 6
	maxGenerateCodeRetries = 5
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GenerateAndAssignReferralCode creates a unique referral code for
// mobileUserID, retrying on code collisions (the unique constraint on
// `code` is the source of truth, not a check-then-insert).
func (s *Service) GenerateAndAssignReferralCode(ctx context.Context, mobileUserID string) error {
	for attempt := 1; attempt <= maxGenerateCodeRetries; attempt++ {
		code, err := GenerateReferralCode(referralCodeLength)
		if err != nil {
			return err
		}
		row := &ReferralCode{ID: uuid.NewString(), Code: code, MobileUserID: mobileUserID}
		err = s.repo.CreateReferralCode(ctx, row)
		if err == nil {
			return nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return err
	}
	return fmt.Errorf("exhausted %d attempts generating a unique referral code for user %s", maxGenerateCodeRetries, mobileUserID)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func (s *Service) RedeemReferralCode(ctx context.Context, mobileUserID, code string) error {
	referral, err := s.repo.FindReferralByCode(ctx, code)
	if err != nil {
		return err
	}
	if referral == nil {
		return nil
	}
	if referral.MobileUserID == mobileUserID {
		return appErr.ErrSelfReferral
	}

	return s.repo.WithTx(ctx, func(r *Repository) error {
		existing, findErr := r.FindRedemptionByReferredUser(ctx, mobileUserID)
		if findErr == nil {
			if existing.ReferrerUserID == referral.MobileUserID {
				return nil
			}
			return appErr.ErrReferralAlreadyRedeemed
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}

		redemption := &ReferralRedemption{
			ID:             uuid.NewString(),
			ReferrerUserID: referral.MobileUserID,
			ReferredUserID: mobileUserID,
			CashbackStatus: CashbackStatusPending,
		}
		if err := r.RedeemReferral(ctx, redemption); err != nil {
			return err
		}

		// Crediting the referrer runs in its own savepoint (WithTx on an
		// already-transactional repo) so a failure here rolls back only the
		// credit attempt, not the redemption itself — registration/redemption
		// must not fail just because the referrer's cashback credit had a
		// transient issue. The redemption is left CashbackStatusPending for
		// RetryPendingReferralCredits to pick up instead of silently losing it.
		if creditErr := r.WithTx(ctx, func(txRepo *Repository) error {
			return s.creditReferralCashback(ctx, txRepo, referral.MobileUserID)
		}); creditErr != nil {
			log.Printf("referrals: cashback credit failed referrer=%s referred=%s: %v", referral.MobileUserID, mobileUserID, creditErr)
			return nil
		}

		return r.UpdateRedemptionCashbackStatus(ctx, redemption.ID, CashbackStatusCredited)
	})
}

// RetryPendingReferralCredits finds redemptions whose referrer cashback
// credit previously failed (CashbackStatusPending) and retries them.
func (s *Service) RetryPendingReferralCredits(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 50
	}
	redemptions, err := s.repo.FindPendingReferralCredits(ctx, limit)
	if err != nil {
		return err
	}
	for _, redemption := range redemptions {
		creditErr := s.repo.WithTx(ctx, func(r *Repository) error {
			return s.creditReferralCashback(ctx, r, redemption.ReferrerUserID)
		})
		if creditErr != nil {
			log.Printf("referrals: retry cashback credit failed redemption=%s referrer=%s: %v", redemption.ID, redemption.ReferrerUserID, creditErr)
			continue
		}
		if err := s.repo.UpdateRedemptionCashbackStatus(ctx, redemption.ID, CashbackStatusCredited); err != nil {
			log.Printf("referrals: failed to mark redemption credited id=%s: %v", redemption.ID, err)
		}
	}
	return nil
}

// creditReferralCashback pays the referrer ReferralCashbackAmountKobo for a
// successful redemption. Cashback is not withdrawable so it never touches the
// wallet's available balance — it is tracked in wallet_cashbacks with running
// totals (cashback_before -> cashback_after) and mirrored by a credit row in
// wallet_transactions whose balance_before/balance_after snapshot the cashback
// ledger.
func (s *Service) creditReferralCashback(ctx context.Context, r *Repository, referrerUserID string) error {
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
	transactionID := uuid.NewString()

	cashback := &models.Cashback{
		ID:             transactionID + "-cashback",
		MobileUserID:   referrerUserID,
		CashbackBefore: before,
		CashbackAfter:  after,
		CashbackAmount: ReferralCashbackAmountKobo,
		Source:         CashbackSourceReferral,
		EntryType:      models.CashbackEntryCredit,
		TransactionID:  &transactionID,
		CreatedAt:      time.Now().UTC(),
	}
	if err := r.CreateCashback(ctx, cashback); err != nil {
		return err
	}

	narration := "Referral cashback"
	txRow := &transaction.Transaction{
		ID:            transactionID,
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
}

func (s *Service) FetchRedeemReferrals(ctx context.Context, page, pageSize int) ([]RedeemedReferral, error) {
	return s.repo.FetchRedeemReferrals(ctx, page, pageSize)
}
