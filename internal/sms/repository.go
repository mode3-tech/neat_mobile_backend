package sms

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateOutgoingSMS(ctx context.Context, id, phone, message, recepient string) (*OutgoingSMS, error) {
	var sms OutgoingSMS

	sms.ID = id
	sms.Phone = phone
	sms.Message = message
	sms.Recepient = recepient
	sms.CreatedAt = time.Now().UTC()

	err := r.db.WithContext(ctx).Create(&sms).Error
	if err != nil {
		return &sms, err
	}
	return &sms, nil
}

func (r *Repository) GetOutgoingSMSByID(ctx context.Context, id string) (*OutgoingSMS, error) {
	var sms OutgoingSMS
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&sms).Error
	if err != nil {
		return nil, err
	}
	return &sms, nil
}

func (r *Repository) UpdateOutgoingSMSStatus(ctx context.Context, id string, status OutgoingSMSStatus, sentAt *time.Time, reasonForFailure string) error {
	var sms OutgoingSMS
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&sms).Error
	if err != nil {
		return err
	}
	sms.Status = status
	sms.SentAt = sentAt
	sms.ReasonForFailure = reasonForFailure
	sms.UpdatedAt = &now
	err = r.db.WithContext(ctx).Save(&sms).Error
	return err
}

func (r *Repository) FetchPendingOutgoingSMS(ctx context.Context, retryBackoff time.Duration) ([]OutgoingSMS, error) {
	var pendingOutgoingSMS []OutgoingSMS
	cutoff := time.Now().UTC().Add(-retryBackoff)
	err := r.db.WithContext(ctx).Where("status = ? OR (status = ? AND retry_count < ? AND last_attempt_at < ?)", "pending", "failed", 5, cutoff).Find(&pendingOutgoingSMS).Error
	if err != nil {
		return nil, err
	}
	return pendingOutgoingSMS, nil
}
