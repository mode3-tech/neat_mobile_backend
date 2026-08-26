package database

import (
	"context"
	"fmt"
	"log"
	"neat_mobile_app_backend/internal/crypto"
	"neat_mobile_app_backend/internal/modules/account"
	"neat_mobile_app_backend/internal/modules/accountclosure"
	"neat_mobile_app_backend/internal/modules/auth"
	"neat_mobile_app_backend/internal/modules/auth/otp"
	"neat_mobile_app_backend/internal/modules/auth/registerv2"
	"neat_mobile_app_backend/internal/modules/autorepayment"
	"neat_mobile_app_backend/internal/modules/card"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/modules/loanproduct"
	"neat_mobile_app_backend/internal/modules/neatsave"
	"neat_mobile_app_backend/internal/modules/referrals"
	"neat_mobile_app_backend/internal/modules/transaction"
	"neat_mobile_app_backend/internal/modules/vas"
	"neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/models"
	"os"
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

	// BVN/NIN encryption at rest: add deterministic hash columns for
	// lookups/dedup (bvn/nin themselves become non-deterministic ciphertext
	// once the application starts writing through internal/crypto.FieldCipher,
	// so equality queries can no longer run against those columns directly).
	// Columns are added nullable and backfilled here - from plaintext, since
	// this runs before the app's write path switches over - then made unique;
	// the app never needs NOT NULL on these since existing rows populated
	// before this migration ran are backfilled in the same pass.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = 'wallet_users'
			) THEN
				ALTER TABLE wallet_users
				ADD COLUMN IF NOT EXISTS bvn_hash text,
				ADD COLUMN IF NOT EXISTS nin_hash text;
			END IF;
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = 'wallet_bvn_records'
			) THEN
				ALTER TABLE wallet_bvn_records
				ADD COLUMN IF NOT EXISTS bvn_hash text;
			END IF;
		END $$;
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		UPDATE wallet_users
		SET bvn_hash = encode(sha256(convert_to(trim(bvn), 'UTF8')), 'hex')
		WHERE bvn_hash IS NULL AND bvn IS NOT NULL AND bvn <> '';
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		UPDATE wallet_users
		SET nin_hash = encode(sha256(convert_to(trim(nin), 'UTF8')), 'hex')
		WHERE nin_hash IS NULL AND nin IS NOT NULL AND nin <> '';
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		UPDATE wallet_bvn_records
		SET bvn_hash = encode(sha256(convert_to(trim(bvn), 'UTF8')), 'hex')
		WHERE bvn_hash IS NULL AND bvn IS NOT NULL AND bvn <> '';
	`).Error; err != nil {
		return err
	}

	// Real uniqueness enforcement moves to the hash columns; the old
	// uq_user_bvn/uq_user_nin indexes (further down) and wallet_bvn_records'
	// original unique index on bvn are left in place but become vestigial
	// once bvn/nin hold ciphertext - AES-GCM's random nonce means no two
	// encryptions of the same value ever collide, so those old indexes can
	// never be violated and are harmless to leave rather than risk dropping
	// an index under a guessed name.
	//
	// These are non-partial (no WHERE ... IS NOT NULL) deliberately: Postgres
	// can only match a plain ON CONFLICT (col) clause (e.g.
	// CreateBVNRecord's, internal/modules/auth/repository.go) against a
	// non-partial unique index, or a partial one whose predicate is repeated
	// verbatim in the ON CONFLICT clause. An earlier partial version of these
	// indexes caused every CreateBVNRecord call to fail with "no unique or
	// exclusion constraint matching the ON CONFLICT specification"
	// (SQLSTATE 42P10) - dropping the predicate isn't a uniqueness regression
	// since Postgres already treats multiple NULLs as non-conflicting under a
	// standard unique index, and bvn_hash/nin_hash are populated for every
	// real row anyway (backfilled above for old rows, always set on new ones).
	// DROP+CREATE (not just CREATE IF NOT EXISTS) so this actually replaces
	// the broken partial index already deployed under this name, not just a
	// no-op on the name match.
	if err := db.Exec(`
		DROP INDEX IF EXISTS uq_wallet_users_bvn_hash;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_users_bvn_hash ON wallet_users(bvn_hash);
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		DROP INDEX IF EXISTS uq_wallet_users_nin_hash;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_users_nin_hash ON wallet_users(nin_hash);
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		DROP INDEX IF EXISTS uq_wallet_bvn_records_bvn_hash;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_bvn_records_bvn_hash ON wallet_bvn_records(bvn_hash);
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
		&accountclosure.AccountClosure{},
		&neatsave.SavingsGoal{},
		&neatsave.AutoSaveRule{},
		&neatsave.SavingsActivity{},
		&autorepayment.AutoRepaymentAttempt{},
		&card.Card{},
		&vas.VASBeneficiary{},
		&accountclosure.AccountClosureEvent{},
		&accountclosure.AccountClosure{},
		&models.ClosureReferenceCounter{},
		&referrals.ReferralCode{},
		&referrals.ReferralRedemption{},
		&models.Cashback{},
		&registerv2.ProviderPreference{},
	); err != nil {
		return err
	}

	// entry_type defaults new rows to 'credit'; AutoMigrate backfills that
	// default onto pre-existing rows too, mislabeling historical VAS-spend
	// (debit) ledger rows. Correct those before the CHECK constraint locks in.
	if err := db.Exec(`
		UPDATE wallet_cashbacks
		SET entry_type = 'debit'
		WHERE entry_type = 'credit' AND cashback_after < cashback_before
	`).Error; err != nil {
		return err
	}

	// cashback_status on referral redemptions defaults new rows to 'pending';
	// AutoMigrate backfills that default onto pre-existing rows too, even
	// though most were already credited under the prior best-effort logic.
	// Mark any redemption whose referrer already has a referral cashback
	// credit as 'credited'; genuinely uncredited ones are left 'pending' so
	// RetryPendingReferralCredits finally pays them out.
	if err := db.Exec(`
		UPDATE wallet_referral_redemptions
		SET cashback_status = 'credited'
		WHERE cashback_status = 'pending'
		  AND EXISTS (
			SELECT 1 FROM wallet_transactions wt
			WHERE wt.mobile_user_id = wallet_referral_redemptions.referrer_user_id
			  AND wt.transaction_category = 'cashback'
			  AND wt.source = 'referral'
		  )
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'wallet_cashbacks_non_negative_balance'
			) THEN
				ALTER TABLE wallet_cashbacks
				ADD CONSTRAINT wallet_cashbacks_non_negative_balance
				CHECK (cashback_before >= 0 AND cashback_after >= 0);
			END IF;
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'wallet_cashbacks_valid_entry_type'
			) THEN
				ALTER TABLE wallet_cashbacks
				ADD CONSTRAINT wallet_cashbacks_valid_entry_type
				CHECK (entry_type IN ('credit', 'debit', 'reversal'));
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

	// Backfill wallet_customer_wallets.provider for rows that predate the
	// column - NUBANs aren't portable between providers, so transfers need to
	// know which one actually issued each account. Matched by bank_code
	// (000036 = Optimus Bank, 100040 = Xpress Wallet/Providus); anything
	// unmatched defaults to providus, matching WALLET_PROVIDER's own default.
	if err := db.Exec(`
		UPDATE wallet_customer_wallets
		SET provider = CASE
			WHEN bank_code = '000036' THEN 'optimus'
			ELSE 'providus'
		END
		WHERE provider IS NULL OR provider = '';
	`).Error; err != nil {
		return err
	}

	// Drop stale mobile_user_id column — an earlier draft of ReferralRedemption
	// used this name before settling on referrer_user_id/referred_user_id.
	if err := db.Exec(`ALTER TABLE wallet_referral_redemptions DROP COLUMN IF EXISTS mobile_user_id`).Error; err != nil {
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

	// wallet_transfers is a legacy table and is not created by AutoMigrate.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema()
				  AND table_name = 'wallet_transfers'
			) THEN
				CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_transfers_transaction_reference
				ON wallet_transfers (transaction_reference)
				WHERE transaction_reference IS NOT NULL AND transaction_reference != '';
			END IF;
		END $$;
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

	if err := db.Exec(`DO $$
		BEGIN
			CREATE UNIQUE INDEX IF NOT EXISTS uq_user_nin ON wallet_users(nin);
			CREATE UNIQUE INDEX IF NOT EXISTS uq_user_bvn ON wallet_users(bvn);
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

	if err := backfillBVNNINEncryption(db); err != nil {
		return err
	}

	return nil
}

// encryptedValuePrefix must match internal/crypto/field.go's versionPrefix -
// it's how backfillEncryptedColumn tells an already-encrypted value apart
// from plaintext still waiting to be backfilled.
const encryptedValuePrefix = "v1:"

const backfillPageSize = 500

// backfillBVNNINEncryption encrypts any wallet_users/wallet_bvn_records/
// wallet_verification_records row still holding plaintext BVN/NIN. The
// earlier hash-column backfill (above, in this same function) could run in
// pure SQL since hashing needs no secret; this can't, since AES-GCM
// encryption needs the app's key, not just the database - so it was deferred
// until now. Self-contained (reads BVN_NIN_ENCRYPTION_KEY directly) rather
// than adding a parameter to Migrate, so every existing caller of Migrate
// (router.go, the cmd/seed-* scripts) keeps compiling unchanged.
func backfillBVNNINEncryption(db *gorm.DB) error {
	cipher, err := crypto.NewFieldCipherFromBase64(os.Getenv("BVN_NIN_ENCRYPTION_KEY"))
	if err != nil {
		return fmt.Errorf("bvn/nin encryption key: %w", err)
	}

	if err := backfillEncryptedColumn(db, cipher, "wallet_users", "bvn"); err != nil {
		return fmt.Errorf("backfill wallet_users.bvn: %w", err)
	}
	if err := backfillEncryptedColumn(db, cipher, "wallet_users", "nin"); err != nil {
		return fmt.Errorf("backfill wallet_users.nin: %w", err)
	}
	if err := backfillEncryptedColumn(db, cipher, "wallet_bvn_records", "bvn"); err != nil {
		return fmt.Errorf("backfill wallet_bvn_records.bvn: %w", err)
	}
	if err := backfillEncryptedColumn(db, cipher, "wallet_verification_records", "verified_id"); err != nil {
		return fmt.Errorf("backfill wallet_verification_records.verified_id: %w", err)
	}
	return nil
}

// backfillEncryptedColumn walks table in keyset-paginated pages (by id, not
// OFFSET, so rows inserted concurrently during the run can't cause other
// rows to be skipped or repeated), encrypting any row whose column is
// non-empty and not already ciphertext. Idempotent - already-encrypted rows
// are skipped, so this is a no-op on every deploy after the first successful
// run. Each row commits independently rather than the whole table in one
// transaction, so this doesn't hold locks against live traffic for its full
// runtime. A single row's encrypt/write failure is logged and skipped rather
// than aborting the run - one bad row shouldn't block the rest or app boot.
func backfillEncryptedColumn(db *gorm.DB, cipher *crypto.FieldCipher, table, column string) error {
	type row struct {
		ID    string
		Value string
	}

	lastID := ""
	scanned := 0
	encryptedCount := 0

	for {
		var rows []row
		query := db.Table(table).
			Select("id, "+column+" AS value").
			Where(column+" IS NOT NULL AND "+column+" != '' AND "+column+" NOT LIKE ?", encryptedValuePrefix+"%")
		if lastID != "" {
			query = query.Where("id > ?", lastID)
		}
		if err := query.Order("id ASC").Limit(backfillPageSize).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}

		for _, r := range rows {
			scanned++
			lastID = r.ID

			ciphertext, err := cipher.Encrypt(r.Value)
			if err != nil {
				log.Printf("backfill %s.%s: encrypt failed id=%s err=%v", table, column, r.ID, err)
				continue
			}

			// Re-checking the old value in WHERE guards against clobbering a
			// row that changed between the read above and this write - low
			// probability since BVN/NIN are effectively write-once after
			// registration, but cheap to guard against.
			result := db.Table(table).
				Where("id = ? AND "+column+" = ?", r.ID, r.Value).
				Update(column, ciphertext)
			if result.Error != nil {
				log.Printf("backfill %s.%s: update failed id=%s err=%v", table, column, r.ID, result.Error)
				continue
			}
			if result.RowsAffected > 0 {
				encryptedCount++
			}
		}

		if len(rows) < backfillPageSize {
			break
		}
	}

	log.Printf("backfill %s.%s: scanned=%d encrypted=%d", table, column, scanned, encryptedCount)
	return nil
}
