package account

import (
	"context"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/device"
	"neat_mobile_app_backend/modules/transaction"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error) {
	var d device.UserDevice
	err := r.db.WithContext(ctx).Where("user_id = ? AND device_id = ?", mobileUserID, deviceID).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) GetUser(ctx context.Context, mobileUserID string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("id = ?", mobileUserID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetAccountSummary(ctx context.Context, mobileUserID string) (*AccountSummaryRow, error) {
	var row AccountSummaryRow
	err := r.db.WithContext(ctx).Model(&models.User{}).
		Select(`wallet_users.id, 
			wallet_users.first_name,
			wallet_users.last_name,
			wallet_users.is_notifications_enabled,
			wallet_users.email,
			wallet_users.dob,
			wallet_users.phone, 
			wallet_bvn_records.bvn,
			wallet_bvn_records.full_home_address AS address,
			wallet_customer_wallets.account_number,
			wallet_customer_wallets.available_balance, 
			wallet_customer_wallets.booked_balance,
			wallet_customer_wallets.bank_name`).
		Joins("LEFT JOIN wallet_bvn_records ON wallet_bvn_records.user_id = wallet_users.id").
		Joins("LEFT JOIN wallet_customer_wallets ON wallet_customer_wallets.mobile_user_id = wallet_users.id").
		Where("wallet_users.id = ?", mobileUserID).Scan(&row).Error
	return &row, err
}

func (r *Repository) UpdateProfile(ctx context.Context, mobileUserID string, req UpdateProfileRequest) error {
	updates := map[string]any{}

	if req.Address != nil {
		updates["email"] = *req.Address
	}

	if req.Email != nil {
		updates["address"] = *req.Email
	}

	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", mobileUserID).
		Updates(updates).Error
}

func (r *Repository) GetStatementTransactions(ctx context.Context, mobileUserID string, walletID string, from, to time.Time) ([]transaction.Transaction, error) {
	var transactions []transaction.Transaction

	err := r.db.WithContext(ctx).
		Where("mobile_user_id = ? AND wallet_id = ? AND created_at >= ? AND created_at <= ? AND status = ?", mobileUserID, walletID, from, to, transaction.TransactionStatusSuccessful).Order("created_at DESC").
		Find(&transactions).Error
	return transactions, err
}

func (r *Repository) CreateAccountReportJob(ctx context.Context, job *AccountReportJob) (*AccountReportJob, error) {
	err := r.db.WithContext(ctx).Create(job).Error
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (r *Repository) GetAccountReportJob(ctx context.Context, jobID string) (*AccountReportJob, error) {
	var job AccountReportJob
	err := r.db.WithContext(ctx).Where("id = ?", jobID).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *Repository) UpdateAccountReportJob(ctx context.Context, job *AccountReportJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

func (r *Repository) GetPendingAccountReportJobs(ctx context.Context) ([]AccountReportJob, error) {
	var jobs []AccountReportJob
	err := r.db.WithContext(ctx).
		Where("status = ?", ReportStatusPending).
		Order("created_at ASC").
		Find(&jobs).Error
	return jobs, err
}

func (r *Repository) ClaimPendingAccountReportJobs(ctx context.Context, limit int) ([]AccountReportJob, error) {
	if limit <= 0 {
		return []AccountReportJob{}, nil
	}

	var jobs []AccountReportJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", ReportStatusPending).
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

		if err := tx.
			Model(&AccountReportJob{}).
			Where("id IN ?", ids).
			Update("status", ReportStatusProcessing).Error; err != nil {
			return err
		}

		for i := range jobs {
			jobs[i].Status = ReportStatusProcessing
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return jobs, nil
}

func (r *Repository) MarkJobProcessing(ctx context.Context, jobID string) error {
	return r.db.WithContext(ctx).
		Model(&AccountReportJob{}).
		Where("id = ? AND status = ?", jobID, ReportStatusPending).
		Update("status", ReportStatusProcessing).Error
}

func (r *Repository) MarkJobFailed(ctx context.Context, jobID string, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&AccountReportJob{}).
		Where("id = ?", jobID).
		Updates(map[string]any{
			"status":    ReportStatusFailed,
			"error_msg": errMsg,
		}).Error
}

func (r *Repository) MarkJobReady(ctx context.Context, jobID string) error {
	return r.db.WithContext(ctx).
		Model(&AccountReportJob{}).
		Where("id = ?", jobID).
		Updates(map[string]any{
			"status": ReportStatusReady,
		}).Error
}

func (r *Repository) SaveDownloadURL(ctx context.Context, jobID, url string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&AccountReportJob{}).
		Where("id = ?", jobID).
		Updates(map[string]any{
			"download_url":   url,
			"url_expires_at": expiresAt,
		}).Error
}
