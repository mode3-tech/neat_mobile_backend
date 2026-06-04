package database

import (
	"context"
	"neat_mobile_app_backend/internal/modules/account"
	"neat_mobile_app_backend/internal/modules/auth"
	"neat_mobile_app_backend/internal/modules/auth/otp"
	"neat_mobile_app_backend/internal/modules/autorepayment"
	"neat_mobile_app_backend/internal/modules/card"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/modules/loanproduct"
	"neat_mobile_app_backend/internal/modules/neatsave"
	"neat_mobile_app_backend/internal/modules/transaction"
	"neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/models"
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

	// Swap back address/email values corrupted by the UpdateProfile column swap bug.
	if err := db.Exec(`
		UPDATE wallet_users
		SET email = address, address = email
		WHERE email NOT LIKE '%@%' AND email != '' AND address IS NOT NULL AND address != '';
	`).Error; err != nil {
		return err
	}

	// Copy notifications_enabled → is_notifications_enabled, then drop the old column.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_users'
				  AND column_name = 'notifications_enabled'
			) THEN
				UPDATE wallet_users
				SET is_notifications_enabled = notifications_enabled
				WHERE is_notifications_enabled IS NULL AND notifications_enabled IS NOT NULL;

				ALTER TABLE wallet_users DROP COLUMN notifications_enabled;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	// Copy password → password_hash for existing rows, then drop the old column.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_users'
				  AND column_name = 'password'
			) THEN
				UPDATE wallet_users
				SET password_hash = password
				WHERE (password_hash IS NULL OR password_hash = '') AND password IS NOT NULL;

				ALTER TABLE wallet_users DROP COLUMN password;
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

	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = 'wallet_users'
			) THEN
				ALTER TABLE wallet_users
				ADD COLUMN IF NOT EXISTS activation_cap_amount bigint NOT NULL DEFAULT 0,
				ADD COLUMN IF NOT EXISTS activation_cap_expires_at timestamptz;
			END IF;
		END $$;
	`).Error; err != nil {
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
		&models.FaceCheckRecord{},
		&auth.RegistrationJob{},
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
		&account.AccountReportJob{},
		&neatsave.SavingsGoal{},
		&neatsave.AutoSaveRule{},
		&neatsave.SavingsActivity{},
		&autorepayment.AutoRepaymentAttempt{},
		&card.Card{},
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
		CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_registration_jobs_phone_open
		ON wallet_registration_jobs (phone)
		WHERE status IN ('pending', 'processing')
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_wallet_registration_jobs_status_created_at
		ON wallet_registration_jobs (status, created_at ASC)
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

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_auto_repayment_attempts_success
		ON wallet_auto_repayment_attempts (loan_repayment_id)
		WHERE status = 'success'
	`).Error; err != nil {
		return err
	}

	// FK: wallet_face_check_records → wallet_verification_records (new table, no existing data)
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_face_check_records'
			) AND NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'fk_wallet_face_check_records_verification'
			) THEN
				ALTER TABLE wallet_face_check_records
				ADD CONSTRAINT fk_wallet_face_check_records_verification
				FOREIGN KEY (verification_record_id) REFERENCES wallet_verification_records(id) ON DELETE CASCADE;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	// FK: wallet_auth_sessions → wallet_users
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_auth_sessions'
			) THEN
				DELETE FROM wallet_auth_sessions
				WHERE NOT EXISTS (
					SELECT 1 FROM wallet_users u WHERE u.id = wallet_auth_sessions.user_id
				);

				IF NOT EXISTS (
					SELECT 1 FROM pg_constraint
					WHERE conname = 'fk_wallet_auth_sessions_user'
				) THEN
					ALTER TABLE wallet_auth_sessions
					ADD CONSTRAINT fk_wallet_auth_sessions_user
					FOREIGN KEY (user_id) REFERENCES wallet_users(id) ON DELETE CASCADE;
				END IF;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	// FK: wallet_user_devices → wallet_users
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_user_devices'
			) THEN
				DELETE FROM wallet_user_devices
				WHERE NOT EXISTS (
					SELECT 1 FROM wallet_users u WHERE u.id = wallet_user_devices.user_id
				);

				IF NOT EXISTS (
					SELECT 1 FROM pg_constraint
					WHERE conname = 'fk_wallet_user_devices_user'
				) THEN
					ALTER TABLE wallet_user_devices
					ADD CONSTRAINT fk_wallet_user_devices_user
					FOREIGN KEY (user_id) REFERENCES wallet_users(id) ON DELETE CASCADE;
				END IF;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	// FK: wallet_device_challenges → wallet_users
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_device_challenges'
			) THEN
				DELETE FROM wallet_device_challenges
				WHERE NOT EXISTS (
					SELECT 1 FROM wallet_users u WHERE u.id = wallet_device_challenges.user_id
				);

				IF NOT EXISTS (
					SELECT 1 FROM pg_constraint
					WHERE conname = 'fk_wallet_device_challenges_user'
				) THEN
					ALTER TABLE wallet_device_challenges
					ADD CONSTRAINT fk_wallet_device_challenges_user
					FOREIGN KEY (user_id) REFERENCES wallet_users(id) ON DELETE CASCADE;
				END IF;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	// FK: wallet_pending_device_sessions → wallet_users
	// user_id was declared as uuid but wallet_users.id is text; cast column type first.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_pending_device_sessions'
			) THEN
				IF EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_schema = current_schema()
					  AND table_name = 'wallet_pending_device_sessions'
					  AND column_name = 'user_id'
					  AND data_type = 'uuid'
				) THEN
					ALTER TABLE wallet_pending_device_sessions
					ALTER COLUMN user_id TYPE text;
				END IF;

				DELETE FROM wallet_pending_device_sessions
				WHERE NOT EXISTS (
					SELECT 1 FROM wallet_users u WHERE u.id = wallet_pending_device_sessions.user_id
				);

				IF NOT EXISTS (
					SELECT 1 FROM pg_constraint
					WHERE conname = 'fk_wallet_pending_device_sessions_user'
				) THEN
					ALTER TABLE wallet_pending_device_sessions
					ADD CONSTRAINT fk_wallet_pending_device_sessions_user
					FOREIGN KEY (user_id) REFERENCES wallet_users(id) ON DELETE CASCADE;
				END IF;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	return nil
}
