package database

import (
	"context"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/auth/otp"
	"neat_mobile_app_backend/modules/device"
	"neat_mobile_app_backend/modules/loanproduct"
	"neat_mobile_app_backend/modules/transaction"
	"neat_mobile_app_backend/modules/wallet"
	"time"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})

	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, err
	}

	return db, nil

}

func Migrate(db *gorm.DB) error {
	// Rename old pin column if it exists and new one does not.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_users'
				  AND column_name = 'pin'
			) AND NOT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_users'
				  AND column_name = 'pin_hash'
			) THEN
				ALTER TABLE wallet_users RENAME COLUMN pin TO pin_hash;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	// Ensure pin_hash exists as nullable before AutoMigrate touches it.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_users'
			) THEN
				ALTER TABLE wallet_users
				ADD COLUMN IF NOT EXISTS pin_hash text;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_users'
			) THEN
				ALTER TABLE wallet_users
				ADD COLUMN IF NOT EXISTS failed_transaction_pin_attempts integer NOT NULL DEFAULT 0,
				ADD COLUMN IF NOT EXISTS transaction_pin_locked_until timestamptz;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	// Drop unique index on provider_reference if it exists — changed to non-unique index.
	if err := db.Exec(`DROP INDEX IF EXISTS idx_wallet_transactions_provider_reference`).Error; err != nil {
		return err
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.BVNRecord{},
		&models.PushToken{},
		&models.Notification{},
		&models.NotificationTicket{},
		&models.AuthSession{},
		&models.RefreshToken{},
		&models.VerificationRecord{},
		&models.PendingDeviceSession{},
		&otp.OTPModel{},
		&device.UserDevice{},
		&device.DeviceChallenge{},
		&loanproduct.LoanProduct{},
		&loanproduct.LoanProductRule{},
		&loanproduct.LoanApplication{},
		&loanproduct.LoanApplicationStatusEvent{},
		&loanproduct.CustomerEvent{},
		&wallet.CustomerWallet{},
		&transaction.Transaction{},
		&wallet.Beneficiary{},
		&wallet.ExpectedDeposit{},
	); err != nil {
		return err
	}

	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_push_tokens'
			) AND NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'fk_wallet_push_tokens_user'
			) THEN
				ALTER TABLE wallet_push_tokens
				ADD CONSTRAINT fk_wallet_push_tokens_user
				FOREIGN KEY (user_id) REFERENCES wallet_users(id) ON DELETE CASCADE;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_notifications'
			) AND NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'fk_wallet_notifications_user'
			) THEN
				ALTER TABLE wallet_notifications
				ADD CONSTRAINT fk_wallet_notifications_user
				FOREIGN KEY (user_id) REFERENCES wallet_users(id) ON DELETE CASCADE;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_wallet_notifications_user_unread
		ON wallet_notifications (user_id, is_read)
		WHERE is_read = FALSE
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_wallet_notifications_user_created_at_desc
		ON wallet_notifications (user_id, created_at DESC)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_notification_tickets'
			) AND NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'fk_wallet_notification_tickets_notification'
			) THEN
				ALTER TABLE wallet_notification_tickets
				ADD CONSTRAINT fk_wallet_notification_tickets_notification
				FOREIGN KEY (notification_id) REFERENCES wallet_notifications(id) ON DELETE CASCADE;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_wallet_notification_tickets_pending
		ON wallet_notification_tickets (created_at, expo_ticket_id)
		WHERE receipt_checked_at IS NULL
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_device_challenges_active
		ON wallet_device_challenges (user_id, device_id)
		WHERE used_at IS NULL
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_wallet_loan_applications_mobile_user_created_at
		ON wallet_loan_applications (mobile_user_id, created_at DESC)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_wallet_loan_applications_embryo_created_at
		ON wallet_loan_applications (created_at DESC)
		WHERE loan_status = 'embryo'
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_wallet_users_bvn
		ON wallet_users (bvn)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_wallet_users_customer_status_id
		ON wallet_users (customer_status, id)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_wallet_users_core_customer_id
		ON wallet_users (core_customer_id)
		WHERE core_customer_id IS NOT NULL
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_transfers_transaction_reference
		ON wallet_transfers (transaction_reference)
		WHERE transaction_reference IS NOT NULL AND transaction_reference != ''
	`).Error; err != nil {
		return err
	}

	return nil
}
