package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const registrationJobLeaseTimeout = 5 * time.Minute

func (r *Repository) CreateRegistrationJob(ctx context.Context, job *RegistrationJob) error {
	if job == nil {
		return gorm.ErrInvalidData
	}

	return r.db.WithContext(ctx).Create(job).Error
}

func (r *Repository) GetRegistrationJobByID(ctx context.Context, jobID string) (*RegistrationJob, error) {
	var job RegistrationJob
	if err := r.db.WithContext(ctx).
		Where("id = ?", strings.TrimSpace(jobID)).
		First(&job).Error; err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *Repository) GetRegistrationJobByIDForUpdate(ctx context.Context, jobID string) (*RegistrationJob, error) {
	var job RegistrationJob
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", strings.TrimSpace(jobID)).
		First(&job).Error; err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *Repository) GetRegistrationJobByIdempotencyKey(ctx context.Context, key string) (*RegistrationJob, error) {
	var job RegistrationJob
	if err := r.db.WithContext(ctx).
		Where("idempotency_key = ?", strings.TrimSpace(key)).
		First(&job).Error; err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *Repository) GetOpenRegistrationJobByPhone(ctx context.Context, phone string) (*RegistrationJob, error) {
	var job RegistrationJob
	if err := r.db.WithContext(ctx).
		Where("phone = ? AND status IN ?", strings.TrimSpace(phone), []RegistrationJobStatus{
			RegistrationJobStatusPending,
			RegistrationJobStatusProcessing,
		}).
		Order("created_at DESC").
		First(&job).Error; err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *Repository) ClaimPendingRegistrationJobs(ctx context.Context, limit int) ([]RegistrationJob, error) {
	if limit <= 0 {
		return []RegistrationJob{}, nil
	}

	var jobs []RegistrationJob
	now := time.Now().UTC()
	staleBefore := now.Add(-registrationJobLeaseTimeout)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&RegistrationJob{}).
			Where("status = ? AND updated_at < ?", RegistrationJobStatusProcessing, staleBefore).
			Updates(map[string]any{
				"status":     RegistrationJobStatusPending,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", RegistrationJobStatusPending).
			Order("created_at ASC").
			Limit(limit).
			Find(&jobs).Error; err != nil {
			return err
		}

		if len(jobs) == 0 {
			return nil
		}

		ids := make([]string, 0, len(jobs))
		for i := range jobs {
			ids = append(ids, jobs[i].ID)
		}

		if err := tx.Model(&RegistrationJob{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":     RegistrationJobStatusProcessing,
				"attempts":   gorm.Expr("attempts + 1"),
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		for i := range jobs {
			jobs[i].Status = RegistrationJobStatusProcessing
			jobs[i].Attempts++
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return jobs, nil
}

func (r *Repository) SaveRegistrationJobWalletResponse(ctx context.Context, jobID, walletResponseJSON string) error {
	if strings.TrimSpace(jobID) == "" {
		return errors.New("registration job id is required")
	}

	return r.db.WithContext(ctx).
		Model(&RegistrationJob{}).
		Where("id = ?", strings.TrimSpace(jobID)).
		Updates(map[string]any{
			"wallet_response_json": walletResponseJSON,
			"last_error":           nil,
		}).Error
}

func (r *Repository) SetRegistrationJobClaimToken(ctx context.Context, jobID, claimTokenHash string, claimExpiresAt time.Time) error {
	jobID = strings.TrimSpace(jobID)
	claimTokenHash = strings.TrimSpace(claimTokenHash)
	if jobID == "" {
		return errors.New("registration job id is required")
	}
	if claimTokenHash == "" {
		return errors.New("registration claim token hash is required")
	}

	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&RegistrationJob{}).
		Where("id = ? AND session_claimed_at IS NULL", jobID).
		Updates(map[string]any{
			"session_claim_token_hash": claimTokenHash,
			"session_claim_expires_at": claimExpiresAt,
			"updated_at":               now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("registration session already claimed")
	}

	return nil
}

func (r *Repository) MarkRegistrationJobCompleted(ctx context.Context, jobID string) error {
	if strings.TrimSpace(jobID) == "" {
		return errors.New("registration job id is required")
	}

	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&RegistrationJob{}).
		Where("id = ?", strings.TrimSpace(jobID)).
		Updates(map[string]any{
			"status":       RegistrationJobStatusCompleted,
			"completed_at": now,
			"last_error":   nil,
			"updated_at":   now,
		}).Error
}

func (r *Repository) MarkRegistrationJobClaimed(ctx context.Context, jobID string, now time.Time) error {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return errors.New("registration job id is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	result := r.db.WithContext(ctx).
		Model(&RegistrationJob{}).
		Where("id = ? AND session_claimed_at IS NULL", jobID).
		Updates(map[string]any{
			"session_claimed_at":       now,
			"session_claim_token_hash": nil,
			"session_claim_expires_at": nil,
			"updated_at":               now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("registration session already claimed")
	}

	return nil
}

func (r *Repository) MarkRegistrationJobFailed(ctx context.Context, jobID, errMsg string) error {
	if strings.TrimSpace(jobID) == "" {
		return errors.New("registration job id is required")
	}

	now := time.Now().UTC()
	trimmedErr := strings.TrimSpace(errMsg)
	return r.db.WithContext(ctx).
		Model(&RegistrationJob{}).
		Where("id = ?", strings.TrimSpace(jobID)).
		Updates(map[string]any{
			"status":     RegistrationJobStatusFailed,
			"last_error": trimmedErr,
			"updated_at": now,
		}).Error
}

func (r *Repository) RequeueRegistrationJob(ctx context.Context, jobID string) error {
	if strings.TrimSpace(jobID) == "" {
		return errors.New("registration job id is required")
	}

	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&RegistrationJob{}).
		Where("id = ?", strings.TrimSpace(jobID)).
		Updates(map[string]any{
			"status":     RegistrationJobStatusPending,
			"last_error": nil,
			"updated_at": now,
		}).Error
}
