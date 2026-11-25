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

func (r *Repository) CreateWemaFaceVerification(ctx context.Context, idType, id, correlationID, status, errorMessage string, now time.Time) error {
	return r.db.WithContext(ctx).Create(&models.WemaFaceVerification{
		ID:            uuid.NewString(),
		IDType:        idType,
		SubjectHash:   wemaFaceSubjectHash(id),
		CorrelationID: correlationID,
		Status:        status,
		ErrorMessage:  errorMessage,
		ExpiresAt:     now.Add(wemaFaceVerificationTTL),
	}).Error
}

// GetLatestWemaFaceVerification returns the newest unexpired result for the
// BVN or NIN, or nil if there is none. status "" matches any result; otherwise
// only that one (registration asks for verified ones that carry a c_id).
func (r *Repository) GetLatestWemaFaceVerification(ctx context.Context, idType, id, status string, now time.Time) (*models.WemaFaceVerification, error) {
	q := r.db.WithContext(ctx).
		Where("id_type = ? AND subject_hash = ? AND expires_at > ?", idType, wemaFaceSubjectHash(id), now)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if status == models.WemaFaceStatusVerified {
		q = q.Where("correlation_id <> ''")
	}
	var rec models.WemaFaceVerification
	result := q.Order("created_at DESC").Limit(1).Find(&rec)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &rec, nil
}
