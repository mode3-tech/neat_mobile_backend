package smsbilling

import (
	"context"
	"log"
	"time"
)

// Config tunes pricing and retry behavior. UnitPriceKobo always comes from
// configuration (there's no sane hardcoded tariff) - see DefaultConfig for
// the retry defaults used in production.
type Config struct {
	UnitPriceKobo     int64
	MaxAttempts       int
	StalePendingAfter time.Duration
	SweepBatchLimit   int
}

func DefaultConfig(unitPriceKobo int64) Config {
	return Config{
		UnitPriceKobo:     unitPriceKobo,
		MaxAttempts:       100,
		StalePendingAfter: 5 * time.Minute,
		SweepBatchLimit:   50,
	}
}

type Service struct {
	Repo   *Repository
	Config Config
}

func NewService(repo *Repository, cfg Config) *Service {
	return &Service{Repo: repo, Config: cfg}
}

// unitsFor approximates SMS segment count the way telcos bill: one segment
// per 160 GSM-7 characters, rounded up. None of the providers wired into this
// codebase (see providers/sms) surface a real segment count from Send(), so
// this is computed client-side rather than left at a hardcoded 1.
func unitsFor(message string) int {
	const segmentLength = 160
	n := len([]rune(message))
	if n == 0 {
		return 1
	}
	units := (n + segmentLength - 1) / segmentLength
	return max(units, 1)
}

// ChargeAndCollect records a billing obligation for one already-sent SMS and
// immediately tries to collect it. Called fire-and-forget right after the
// provider accepts the message (see BillableSender.SendBillable) - it must
// never be able to affect the outcome of the send itself, so every failure
// here is logged and swallowed. CollectOutstanding (after the next wallet
// credit) and the reconciliation sweep are the backstops for anything left
// pending.
func (s *Service) ChargeAndCollect(ctx context.Context, mobileUserID, phone, message, purpose string) {
	charge, err := s.Repo.CreateCharge(ctx, mobileUserID, phone, purpose, unitsFor(message), s.Config.UnitPriceKobo)
	if err != nil {
		log.Printf("smsbilling: failed to record charge: user=%s purpose=%s err=%v", mobileUserID, purpose, err)
		return
	}
	if _, err := s.Repo.AttemptCollect(ctx, charge.ID, s.Config.MaxAttempts); err != nil {
		log.Printf("smsbilling: initial collection attempt failed: charge=%s err=%v", charge.ID, err)
	}
}

// CollectOutstanding retries a user's outstanding SMS charges. This is the
// fast-recovery path called after a wallet credit lands (see
// wallet.Service.ProcessAccountFunded) - bounded to a small batch since it
// runs inline with (a goroutine off) the credit webhook.
func (s *Service) CollectOutstanding(ctx context.Context, mobileUserID string) {
	ids, err := s.Repo.ListOutstandingForUser(ctx, mobileUserID, 20)
	if err != nil {
		log.Printf("smsbilling: failed to list outstanding charges: user=%s err=%v", mobileUserID, err)
		return
	}
	for _, id := range ids {
		if _, err := s.Repo.AttemptCollect(ctx, id, s.Config.MaxAttempts); err != nil {
			log.Printf("smsbilling: credit-triggered collection failed: charge=%s err=%v", id, err)
		}
	}
}

// ReconcileOutstanding is the cron entrypoint - the safety net for charges
// missed by the immediate attempt and credit-triggered recovery (process
// restarts, crashes, wallets funded through a path other than
// ProcessAccountFunded).
func (s *Service) ReconcileOutstanding(ctx context.Context) error {
	ids, err := s.Repo.ListOutstandingCandidates(ctx, s.Config.SweepBatchLimit, s.Config.StalePendingAfter, s.Config.MaxAttempts)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := s.Repo.AttemptCollect(ctx, id, s.Config.MaxAttempts); err != nil {
			log.Printf("smsbilling: sweep collection failed: charge=%s err=%v", id, err)
		}
	}
	return nil
}
