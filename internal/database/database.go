package database

import (
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/auth/otp"
	"neat_mobile_app_backend/modules/device"
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

	return db, nil

}

func Migrate(db *gorm.DB) error {
	// Keep production data intact when migrating from old schema that used `pin`.
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

	if err := db.AutoMigrate(
		&models.User{},
		&models.AuthSession{},
		&models.RefreshToken{},
		&models.VerificationRecord{},
		&models.PendingDeviceSession{},
		&otp.OTPModel{},
		&device.UserDevice{},
		&device.DeviceChallenge{},
	); err != nil {
		return err
	}

	// Enforce one active (unused) challenge per user/device even under concurrency.
	return db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_device_challenges_active
		ON device_challenges (user_id, device_id)
		WHERE used_at IS NULL
	`).Error
}
