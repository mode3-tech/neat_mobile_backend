package loanproduct

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InternalRepository struct {
	db *gorm.DB
}

func NewInternalRepository(db *gorm.DB) *InternalRepository {
	return &InternalRepository{db: db}
}

func (r *InternalRepository) WithTx(ctx context.Context, fn func(*InternalRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewInternalRepository(tx))
	})
}

func (r *InternalRepository) GetApplicationByRefForUpdate(ctx context.Context, ref string) (*LoanApplication, error) {
	var row LoanApplication
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("application_ref = ?", ref).
		First(&row).Error
	return &row, err
}

func (r *InternalRepository) InsertStatusEvent(ctx context.Context, ev *LoanApplicationStatusEvent) (bool, error) {
	tx := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).
		Create(ev)
	return tx.RowsAffected == 1, tx.Error
}

func (r *InternalRepository) UpdateApplicationStatus(ctx context.Context, ref string, status LoanStatus, coreLoanID *string, now time.Time) error {
	updates := map[string]any{
		"loan_status": status,
		"updated_at":  now,
	}
	if coreLoanID != nil {
		updates["core_loan_id"] = *coreLoanID
	}

	return r.db.WithContext(ctx).
		Model(&LoanApplication{}).
		Where("application_ref = ?", ref).
		Updates(updates).Error
}
