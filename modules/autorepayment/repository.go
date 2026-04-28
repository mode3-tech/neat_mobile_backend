package autorepayment

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

const dueTodayQuery = `
SELECT
	lr.id AS repayment_id,
	lr.loan_id::text AS loan_id,
	lr.amount AS amount,
	wu.id AS mobile_user_id,
	wu.core_customer_id
FROM loan_loan_repayment lr
JOIN loan_loan l ON l.id = lr.loan_id
JOIN account_customer_info aci ON aci.user_id = l.customer_id
JOIN wallet_users wu ON wu.core_customer_id = aci.id
WHERE lr.paid IS NOT TRUE
  AND lr.expected_to_be_paid_date::date <=CURRENT_DATE
  AND l.status = 'Active'
  AND NOT EXISTS (
	  SELECT 1 FROM wallet_auto_repayment_attempts a
	  WHERE a.loan_repayment_id = lr.id
	  	AND a.status = 'success'
  )
ORDER BY lr.expected_to_be_paid_date
`

func (r *Repository) GetDueRepayments(ctx context.Context) ([]DueRepaymentRow, error) {
	var dueRepayments []DueRepaymentRow
	err := r.db.WithContext(ctx).Raw(dueTodayQuery).Scan(&dueRepayments).Error
	return dueRepayments, err
}

func (r *Repository) HasActiveAttempt(ctx context.Context, loanRepaymentID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&AutoRepaymentAttempt{}).
		Where("loan_repayment_id = ? AND status = ?", loanRepaymentID, AutoRepaymentAttemptStatusPending).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) InsertAttempt(ctx context.Context, attempt *AutoRepaymentAttempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *Repository) UpdateAttemptStatus(ctx context.Context, id string, status AutoRepaymentAttemptStatus, failureReason, providerRef string) error {
	return r.db.WithContext(ctx).
		Model(&AutoRepaymentAttempt{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":         status,
			"failure_reason": failureReason,
			"provider_ref":   providerRef,
		}).Error
}
