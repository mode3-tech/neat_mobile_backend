package verification

import (
	"context"
	"errors"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm/clause"

	"gorm.io/gorm"
)

type VerificationRepo struct {
	db *gorm.DB
}

func NewVerification(db *gorm.DB) *VerificationRepo {
	return &VerificationRepo{db: db}
}

func (r *VerificationRepo) AddVerification(ctx context.Context, verification *models.VerificationRecord) error {
	return r.db.WithContext(ctx).Create(verification).Error
}

func (r *VerificationRepo) GetVerificationByID(ctx context.Context, id string) (*models.VerificationRecord, error) {
	var rec models.VerificationRecord
	result := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND status = ?", id, models.VerificationStatusPending).
		Limit(1).
		Find(&rec)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &rec, nil
}

func (r *VerificationRepo) MarkVerificationUsed(ctx context.Context, id string, usedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.VerificationRecord{}).
		Where("id = ? AND status = ?", id, models.VerificationStatusVerified).
		Updates(map[string]any{"status": models.VerificationStatusUsed, "used_at": usedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("verification record already used or not found")
	}
	return nil
}
