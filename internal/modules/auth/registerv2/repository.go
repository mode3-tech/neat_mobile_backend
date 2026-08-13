package registerv2

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
	return &Repository{
		db: db,
	}
}

// GetVerificationByID looks up a verification record by its ID, locking the
// row for update - mirrors verification.VerificationRepo.GetVerificationByID.
func (r *Repository) GetVerificationByID(ctx context.Context, id string) (*models.VerificationRecord, error) {
	var rec models.VerificationRecord
	result := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
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

// GetVerifiedByTypeAndHash finds a verified record for an identity number
// (BVN/NIN) by its subject hash - there's no client-facing ID for these, the
// client resends the raw value and we re-derive the hash to look it up.
func (r *Repository) GetVerifiedByTypeAndHash(ctx context.Context, verificationType, subjectHash string) (*models.VerificationRecord, error) {
	var rec models.VerificationRecord
	result := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("type = ? AND subject_hash = ? AND status = ?", verificationType, subjectHash, models.VerificationStatusVerified).
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

// MarkVerificationUsed mirrors verification.VerificationRepo.MarkVerificationUsed.
func (r *Repository) MarkVerificationUsed(ctx context.Context, id string, usedAt time.Time) error {
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

// GetProviderPreference returns the current wallet provider preference row.
func (r *Repository) GetProviderPreference(ctx context.Context) (*ProviderPreference, error) {
	var pref ProviderPreference
	if err := r.db.WithContext(ctx).First(&pref, "id = ?", 1).Error; err != nil {
		return nil, err
	}
	return &pref, nil
}

func (r *Repository) AddVerification(ctx context.Context, record *models.VerificationRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}
