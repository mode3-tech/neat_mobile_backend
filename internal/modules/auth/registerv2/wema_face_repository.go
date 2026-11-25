package registerv2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"neat_mobile_app_backend/models"

	"github.com/google/uuid"
)

// wemaFaceVerificationTTL is how long a face verification from Wema's web app
// stays usable for creating the account.
const wemaFaceVerificationTTL = 24 * time.Hour

// wemaFaceSubjectHash is the hash a WemaFaceVerification stores for a BVN or
// NIN.
func wemaFaceSubjectHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func (r *Repository) CreateWemaFaceVerification(ctx context.Context, idType, id, correlationID string, now time.Time) error {
	return r.db.WithContext(ctx).Create(&models.WemaFaceVerification{
		ID:            uuid.NewString(),
		IDType:        idType,
		SubjectHash:   wemaFaceSubjectHash(id),
		CorrelationID: correlationID,
		ExpiresAt:     now.Add(wemaFaceVerificationTTL),
	}).Error
}

// GetLatestWemaFaceVerification returns the newest unexpired
// verification for the BVN or NIN, or nil if there is none.
func (r *Repository) GetLatestWemaFaceVerification(ctx context.Context, idType, id string, now time.Time) (*models.WemaFaceVerification, error) {
	var rec models.WemaFaceVerification
	result := r.db.WithContext(ctx).
		Where("id_type = ? AND subject_hash = ? AND expires_at > ?", idType, wemaFaceSubjectHash(id), now).
		Order("created_at DESC").
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
