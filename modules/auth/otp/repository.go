package otp

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OTPRepository struct {
	db *gorm.DB
}

func NewOTPRepository(db *gorm.DB) *OTPRepository {
	return &OTPRepository{db: db}
}

func (o *OTPRepository) CreateOTP(ctx context.Context, otp *OTPModel) error {
	return o.db.WithContext(ctx).Create(otp).Error
}

func (o *OTPRepository) GetActiveOTP(ctx context.Context, destination string, purpose Purpose) (*OTPModel, error) {
	var otp OTPModel
	result := o.db.WithContext(ctx).
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

func (o *OTPRepository) UpdateForResend(ctx context.Context, id string, newHash string, newExp time.Time, nextSendAt time.Time) error {
	return o.db.WithContext(ctx).
		Model(&OTPModel{}).
		Where("id = ? AND consumed_at IS NULL", id).
		Updates(map[string]any{
			"otp_hash":     newHash,
			"expires_at":   newExp,
			"next_send_at": nextSendAt,
			"resend_count": gorm.Expr("resend_count + 1"),
		}).Error
}

func (o *OTPRepository) ConsumeOTP(ctx context.Context, id string, consumedAt time.Time) error {
	result := o.db.WithContext(ctx).Model(&OTPModel{}).Where("id = ? AND consumed_at IS NULL", id).Update("consumed_at", consumedAt)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("otp already used or not found")
	}

	return nil
}

func (o *OTPRepository) IncrementAttempt(ctx context.Context, id string) error {
	return o.db.WithContext(ctx).
		Model(&OTPModel{}).
		Where("id = ? AND consumed_at IS NULL", id).
		Update("attempt_count", gorm.Expr("attempt_count + 1")).Error
}

func (o *OTPRepository) WithTx(ctx context.Context, fn func(r *OTPRepository) error) error {
	return o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&OTPRepository{db: tx})
	})
}
