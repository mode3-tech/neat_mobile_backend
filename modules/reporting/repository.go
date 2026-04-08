package reporting

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

const slowQueryThreshold = 2 * time.Second

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) logQuery(name string, startedAt time.Time, detail string, err error) {
	duration := time.Since(startedAt)
	if err == nil && duration < slowQueryThreshold {
		return
	}

	message := fmt.Sprintf("reporting query=%s duration=%s", name, duration.Round(time.Millisecond))
	if detail != "" {
		message += " " + detail
	}

	if sqlDB, dbErr := r.db.DB(); dbErr == nil {
		stats := sqlDB.Stats()
		message += fmt.Sprintf(
			" db_open=%d db_in_use=%d db_idle=%d",
			stats.OpenConnections,
			stats.InUse,
			stats.Idle,
		)
	}

	if err != nil {
		log.Printf("%s err=%v", message, err)
		return
	}
	log.Print(message)
}

func (r *Repository) ListSignedUsers(ctx context.Context, limit, offset int) ([]signedUserRow, int64, error) {
	var total int64
	var rows []signedUserRow

	base := r.db.WithContext(ctx).
		Table("wallet_users wu").
		Joins("LEFT JOIN wallet_bvn_records wbr ON wbr.bvn = wu.bvn").
		Joins(`LEFT JOIN LATERAL (
			SELECT loan_status, created_at
			FROM wallet_loan_applications
			WHERE mobile_user_id = wu.id
			ORDER BY created_at DESC
			LIMIT 1
		) latest_loan ON true`)

	countStart := time.Now()
	if err := base.Count(&total).Error; err != nil {
		r.logQuery("ListSignedUsers.count", countStart, "", err)
		return nil, 0, err
	}
	r.logQuery("ListSignedUsers.count", countStart, fmt.Sprintf("total=%d", total), nil)

	if total == 0 {
		return []signedUserRow{}, 0, nil
	}

	listStart := time.Now()
	err := base.
		Select(`
			wu.id,
			wu.first_name,
			wu.last_name,
			wu.middle_name,
			wu.email,
			wu.phone,
			wu.bvn,
			wu.core_customer_id,
			wu.customer_status,
			wu.username,
			wu.is_bvn_verified,
			wu.is_nin_verified,
			wu.is_phone_verified,
			wu.created_at,
			wbr.first_name AS bvn_first_name,
			wbr.last_name  AS bvn_last_name,
			latest_loan.loan_status,
			latest_loan.created_at AS last_loan_applied_at
		`).
		Order("wu.created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	r.logQuery("ListSignedUsers.list", listStart, fmt.Sprintf("limit=%d offset=%d rows=%d", limit, offset, len(rows)), err)
	if err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}
