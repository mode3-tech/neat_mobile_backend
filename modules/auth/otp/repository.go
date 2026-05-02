package otp

import (
	"context"
	"errors"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateOTP(ctx context.Context, otp *OTPModel) error {
	return r.db.WithContext(ctx).Create(otp).Error
}

func (r *Repository) GetActiveOTP(ctx context.Context, destination string, purpose Purpose) (*OTPModel, error) {
	var otp OTPModel
	result := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("destination = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ?", destination, purpose, time.Now().UTC()).
		Limit(1).
		Find(&otp)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &otp, nil
}

func (r *Repository) GetActiveOTPByID(ctx context.Context, id string, purpose Purpose) (*OTPModel, error) {
	var otp OTPModel
	result := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ?", id, purpose, time.Now().UTC()).
		Limit(1).
		Find(&otp)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}

	return &otp, nil
}

func (r *Repository) UpdateForResend(ctx context.Context, id string, newHash string, newExp time.Time, nextSendAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&OTPModel{}).
		Where("id = ? AND consumed_at IS NULL", id).
		Updates(map[string]any{
			"otp_hash":      newHash,
			"expires_at":    newExp,
			"next_send_at":  nextSendAt,
			"attempt_count": 0,
			"resend_count":  gorm.Expr("resend_count + 1"),
		}).Error
}

func (r *Repository) ConsumeOTP(ctx context.Context, id string, consumedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&OTPModel{}).Where("id = ? AND consumed_at IS NULL", id).Update("consumed_at", consumedAt)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("otp already used or not found")
	}

	return nil
}

func (r *Repository) IncrementAttempt(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&OTPModel{}).
		Where("id = ? AND consumed_at IS NULL", id).
		Update("attempt_count", gorm.Expr("attempt_count + 1")).Error
}

func (r *Repository) WithTx(ctx context.Context, fn func(r *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}

func (r *Repository) GetVerificationRow(ctx context.Context, verificationID string) (*models.VerificationRecord, error) {
	var verification models.VerificationRecord
	result := r.db.WithContext(ctx).Where("id = ? AND used_at IS NULL", verificationID).Find(&verification)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &verification, nil
}
