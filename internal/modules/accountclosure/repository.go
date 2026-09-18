package accountclosure

import (
	"context"
	"errors"
	"neat_mobile_app_backend/internal/types"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateAccountClosureEvent(ctx context.Context, event *AccountClosureEvent) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return err
	}
	return nil
}

func (r *Repository) CreateAccountClosure(ctx context.Context, accountClosure *AccountClosure) error {
	if err := r.db.WithContext(ctx).Create(accountClosure).Error; err != nil {
		return err
	}
	return nil
}

func (r *Repository) MarkAccountClosureAsBlocked(ctx context.Context, accountClosureID string, blockerCode string, blockerDetails types.JSONMap) error {
	tx := r.db.WithContext(ctx).Model(&AccountClosure{}).Where("id = ? AND status = ?", accountClosureID, AccountClosureStatusChecking).
		Updates(map[string]any{
			"status":          AccountClosureStatusBlocked,
			"blocker_code":    blockerCode,
			"blocker_details": blockerDetails,
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return errors.New("account closure was not found or is not in checking state")
	}
	return nil
}

func (r *Repository) MarkAccountClosureAsProcessing(ctx context.Context, accountClosureID string) error {
	tx := r.db.WithContext(ctx).Model(&AccountClosure{}).Where("id = ? AND status = ?", accountClosureID, AccountClosureStatusChecking).
		Updates(map[string]any{
			"status": AccountClosureStatusProcessing,
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return errors.New("account closure was not found or is not in checking state")
	}
	return nil
}

func (r *Repository) MarkAccountClosureAsClosed(ctx context.Context, accountClosureID string, completedAt time.Time, providerStatus string) error {
	tx := r.db.WithContext(ctx).Model(&AccountClosure{}).Where("id = ? AND status = ?", accountClosureID, AccountClosureStatusProcessing).
		Updates(map[string]any{
			"status":          AccountClosureStatusClosed,
			"completed_at":    completedAt,
			"provider_status": providerStatus,
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return errors.New("account closure was not found or is not in processing state")
	}
	return nil
}

func (r *Repository) GetActiveClosureForUser(ctx context.Context, userID string) (*AccountClosure, error) {
	var closure AccountClosure
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status IN ?", userID, []AccountClosureStatus{AccountClosureStatusChecking, AccountClosureStatusProcessing}).
		First(&closure).Error
	if err != nil {
		return nil, err
	}
	return &closure, nil
}

func (r *Repository) GetNextClosureSequence(
	ctx context.Context,
	period string,
) (int64, error) {
	var sequence int64

	tx := r.db.WithContext(ctx).Raw(`
		INSERT INTO wallet_account_closure_reference_counters (
			period,
			last_sequence,
			created_at,
			updated_at
		)
		VALUES (?, 1, NOW(), NOW())
		ON CONFLICT (period)
		DO UPDATE SET
			last_sequence = wallet_account_closure_reference_counters.last_sequence + 1,
			updated_at = NOW()
		RETURNING last_sequence
	`, period).Scan(&sequence)

	if tx.Error != nil {
		return 0, tx.Error
	}

	return sequence, nil
}
