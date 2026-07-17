package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/internal/modules/referrals"
)

const (
	referralCodeLength = 6
	maxGenerateRetries = 5
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	if strings.TrimSpace(cfg.DBUrl) == "" {
		log.Fatal("DB_URL is required")
	}

	db, err := database.NewPostgres(cfg.DBUrl)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()

	var userIDs []string
	if err := db.WithContext(ctx).
		Table("wallet_users AS u").
		Where("NOT EXISTS (SELECT 1 FROM wallet_referral_codes rc WHERE rc.mobile_user_id = u.id)").
		Pluck("u.id", &userIDs).Error; err != nil {
		log.Fatalf("failed to list users without a referral code: %v", err)
	}

	if len(userIDs) == 0 {
		log.Println("every user already has a referral code — nothing to do")
		return
	}

	log.Printf("backfilling referral codes for %d user(s)", len(userIDs))

	var created, failed int
	for _, userID := range userIDs {
		if err := createReferralCodeWithRetry(ctx, db, userID); err != nil {
			log.Printf("failed to create referral code for user %s: %v", userID, err)
			failed++
			continue
		}
		created++
	}

	log.Printf("done: created=%d failed=%d", created, failed)
}

// createReferralCodeWithRetry generates a code and attempts to insert it,
// relying on the DB's unique constraint on `code` as the source of truth for
// collisions (insert-and-retry, not check-then-insert — see GenerateReferralCode).
func createReferralCodeWithRetry(ctx context.Context, db *gorm.DB, userID string) error {
	for attempt := 1; attempt <= maxGenerateRetries; attempt++ {
		code, err := referrals.GenerateReferralCode(referralCodeLength)
		if err != nil {
			return err
		}

		row := &referrals.ReferralCode{
			ID:           uuid.NewString(),
			Code:         code,
			MobileUserID: userID,
		}

		err = db.WithContext(ctx).Create(row).Error
		if err == nil {
			return nil
		}

		if isUniqueViolation(err) {
			log.Printf("code collision for user %s (attempt %d/%d), regenerating", userID, attempt, maxGenerateRetries)
			continue
		}

		return err
	}

	return fmt.Errorf("exhausted %d attempts generating a unique referral code for user %s", maxGenerateRetries, userID)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
