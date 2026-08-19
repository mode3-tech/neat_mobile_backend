package registerv2

import (
	"context"
	"errors"
	"neat_mobile_app_backend/internal/crypto"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db     *gorm.DB
	cipher *crypto.FieldCipher
}

func NewRepository(db *gorm.DB, cipher *crypto.FieldCipher) *Repository {
	return &Repository{
		db:     db,
		cipher: cipher,
	}
}

// decryptVerifiedID reverses AddVerification's encryption of VerifiedID.
// Safe on rows written before encryption was introduced - Decrypt passes
// legacy plaintext through unchanged.
func (r *Repository) decryptVerifiedID(rec *models.VerificationRecord) error {
	if rec == nil || rec.VerifiedID == nil {
		return nil
	}
	plain, err := r.cipher.Decrypt(*rec.VerifiedID)
	if err != nil {
		return err
	}
	rec.VerifiedID = &plain
	return nil
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
	if err := r.decryptVerifiedID(&rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// GetLatestByTypeAndHash returns the most recent verification record for an
// identity number (BVN/NIN) by its subject hash, regardless of status - there's
// no client-facing ID for these, the client resends the raw value and we
// re-derive the hash to look it up. Register uses the status to tell apart
// "never attempted" (nil), "Optimus rejected it" (failed - fallback eligible),
// "OTP not yet confirmed" (pending - not fallback eligible), and "verified".
func (r *Repository) GetLatestByTypeAndHash(ctx context.Context, verificationType, subjectHash string) (*models.VerificationRecord, error) {
	var rec models.VerificationRecord
	result := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("type = ? AND subject_hash = ?", verificationType, subjectHash).
		Order("created_at DESC").
		Limit(1).
		Find(&rec)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	if err := r.decryptVerifiedID(&rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// MarkVerificationVerifiedByProviderReference flips a pending BVN/NIN
// verification record to verified once its Optimus OTP challenge is
// confirmed - looked up by the provider's reference id, since that's the
// only identifier VerifyOTP has to go on.
func (r *Repository) MarkVerificationVerifiedByProviderReference(ctx context.Context, providerReferenceID string, verifiedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.VerificationRecord{}).
		Where("provider_verification_id = ? AND status = ?", providerReferenceID, models.VerificationStatusPending).
		Updates(map[string]any{"status": models.VerificationStatusVerified, "verified_at": verifiedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("verification record not found or already verified")
	}
	return nil
}

// UpdateProviderVerificationReference repoints a pending verification record
// from an old Optimus OTP reference id to the new one issued by a resend -
// without this, VerifyOTP's lookup by reference id would miss the record,
// since the client is required to confirm against the new id, not the old one.
func (r *Repository) UpdateProviderVerificationReference(ctx context.Context, oldReferenceID, newReferenceID string) error {
	result := r.db.WithContext(ctx).
		Model(&models.VerificationRecord{}).
		Where("provider_verification_id = ? AND status = ?", oldReferenceID, models.VerificationStatusPending).
		Update("provider_verification_id", newReferenceID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("verification record not found or already verified")
	}
	return nil
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
	if record.VerifiedID == nil {
		return r.db.WithContext(ctx).Create(record).Error
	}
	// Encrypt into a copy so the caller's struct keeps its plaintext
	// VerifiedID - the caller (registerv2/service.go) doesn't reuse it after
	// this call today, but matching the same defensive pattern used in
	// auth.Repository's CreateUser/CreateBVNRecord keeps this safe regardless.
	toInsert := *record
	encrypted, err := r.cipher.Encrypt(*record.VerifiedID)
	if err != nil {
		return err
	}
	toInsert.VerifiedID = &encrypted
	return r.db.WithContext(ctx).Create(&toInsert).Error
}
