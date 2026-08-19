package verification

import (
	"context"
	"errors"
	"neat_mobile_app_backend/internal/crypto"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm/clause"

	"gorm.io/gorm"
)

type VerificationRepo struct {
	db     *gorm.DB
	cipher *crypto.FieldCipher
}

func NewVerification(db *gorm.DB, cipher *crypto.FieldCipher) *VerificationRepo {
	return &VerificationRepo{db: db, cipher: cipher}
}

// Cipher exposes the field cipher this repo was constructed with, so callers
// that need a fresh tx-scoped VerificationRepo (e.g. otp.Service inside a
// WithTx block) can reuse the same cipher instance without redeclaring it.
func (r *VerificationRepo) Cipher() *crypto.FieldCipher {
	return r.cipher
}

// AddVerification persists a verification record, encrypting VerifiedID
// (holds a raw BVN/NIN for BVN/NIN checks) if present. See the matching
// comment in registerv2/repository.go's AddVerification for why encryption
// happens into a copy rather than mutating the caller's struct.
func (r *VerificationRepo) AddVerification(ctx context.Context, verification *models.VerificationRecord) error {
	if verification.VerifiedID == nil {
		return r.db.WithContext(ctx).Create(verification).Error
	}
	toInsert := *verification
	encrypted, err := r.cipher.Encrypt(*verification.VerifiedID)
	if err != nil {
		return err
	}
	toInsert.VerifiedID = &encrypted
	return r.db.WithContext(ctx).Create(&toInsert).Error
}

func (r *VerificationRepo) GetVerificationByID(ctx context.Context, id string) (*models.VerificationRecord, error) {
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
	if rec.VerifiedID != nil {
		plain, err := r.cipher.Decrypt(*rec.VerifiedID)
		if err != nil {
			return nil, err
		}
		rec.VerifiedID = &plain
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
