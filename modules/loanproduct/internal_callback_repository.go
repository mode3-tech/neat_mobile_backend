package loanproduct

import (
	"context"
	"errors"
	"fmt"
	"log"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const cbaApplicationSelectColumns = `
	wallet_loan_applications.application_ref,
	wallet_loan_applications.mobile_user_id,
	wallet_loan_applications.core_customer_id AS application_core_customer_id,
	wallet_users.core_customer_id AS user_core_customer_id,
	wallet_users.customer_status AS user_customer_status,
	wallet_loan_applications.phone_number,
	wallet_loan_applications.loan_product_type,
	wallet_loan_applications.business_address,
	wallet_loan_applications.business_value,
	wallet_loan_applications.business_type,
	wallet_loan_applications.requested_amount,
	wallet_loan_applications.loan_status,
	wallet_loan_applications.tenure,
	wallet_loan_applications.tenure_value,
	wallet_bvn_records.id AS bvn_record_id,
	wallet_bvn_records.bvn,
	wallet_bvn_records.first_name,
	wallet_bvn_records.middle_name,
	wallet_bvn_records.last_name,
	wallet_bvn_records.gender,
	wallet_bvn_records.nationality,
	wallet_bvn_records.state_of_origin,
	wallet_bvn_records.date_of_birth,
	wallet_bvn_records.email_address,
	wallet_bvn_records.mobile_phone,
	wallet_bvn_records.alternative_mobile_phone,
	wallet_bvn_records.bank_name,
	wallet_bvn_records.full_home_address,
	wallet_bvn_records.passport_on_bvn,
	wallet_bvn_records.city,
	wallet_bvn_records.landmark
`

type InternalRepository struct {
	db *gorm.DB
}

const (
	defaultEmbryoLoanApplicationsPage  = 1
	defaultEmbryoLoanApplicationsLimit = 10
	maxEmbryoLoanApplicationsLimit     = 100
	internalCallbackSlowQueryThreshold = 2 * time.Second
)

type cbaApplicationReadRow struct {
	ApplicationRef            string     `gorm:"column:application_ref"`
	MobileUserID              string     `gorm:"column:mobile_user_id"`
	ApplicationCoreCustomerID *string    `gorm:"column:application_core_customer_id"`
	UserCoreCustomerID        *string    `gorm:"column:user_core_customer_id"`
	UserCustomerStatus        *string    `gorm:"column:user_customer_status"`
	PhoneNumber               string     `gorm:"column:phone_number"`
	LoanProductType           string     `gorm:"column:loan_product_type"`
	BusinessAddress           string     `gorm:"column:business_address"`
	BusinessValue             int64      `gorm:"column:business_value"`
	BusinessType              string     `gorm:"column:business_type"`
	RequestedAmount           int64      `gorm:"column:requested_amount"`
	LoanStatus                string     `gorm:"column:loan_status"`
	Tenure                    string     `gorm:"column:tenure"`
	TenureValue               int        `gorm:"column:tenure_value"`
	BVNRecordID               *string    `gorm:"column:bvn_record_id"`
	BVN                       *string    `gorm:"column:bvn"`
	FirstName                 *string    `gorm:"column:first_name"`
	MiddleName                *string    `gorm:"column:middle_name"`
	LastName                  *string    `gorm:"column:last_name"`
	Gender                    *string    `gorm:"column:gender"`
	Nationality               *string    `gorm:"column:nationality"`
	StateOfOrigin             *string    `gorm:"column:state_of_origin"`
	DateOfBirth               *time.Time `gorm:"column:date_of_birth"`
	EmailAddress              *string    `gorm:"column:email_address"`
	MobilePhone               *string    `gorm:"column:mobile_phone"`
	AlternativeMobilePhone    *string    `gorm:"column:alternative_mobile_phone"`
	BankName                  *string    `gorm:"column:bank_name"`
	FullHomeAddress           *string    `gorm:"column:full_home_address"`
	PassportOnBVN             *string    `gorm:"column:passport_on_bvn"`
	City                      *string    `gorm:"column:city"`
	Landmark                  *string    `gorm:"column:landmark"`
}

type cbaEmbryoApplicationSummaryRow struct {
	ApplicationRef string  `gorm:"column:application_ref"`
	MobileUserID   string  `gorm:"column:mobile_user_id"`
	PhoneNumber    string  `gorm:"column:phone_number"`
	FirstName      *string `gorm:"column:first_name"`
	MiddleName     *string `gorm:"column:middle_name"`
	LastName       *string `gorm:"column:last_name"`
	Gender         *string `gorm:"column:gender"`
	LoanStatus     string  `gorm:"column:loan_status"`
	CustomerStatus *string `gorm:"column:customer_status"`
}

type cbaBVNRecordReadRow struct {
	ApplicationRef         string     `gorm:"column:application_ref"`
	BVN                    *string    `gorm:"column:bvn"`
	FirstName              *string    `gorm:"column:first_name"`
	MiddleName             *string    `gorm:"column:middle_name"`
	LastName               *string    `gorm:"column:last_name"`
	Gender                 *string    `gorm:"column:gender"`
	Nationality            *string    `gorm:"column:nationality"`
	StateOfOrigin          *string    `gorm:"column:state_of_origin"`
	DateOfBirth            *time.Time `gorm:"column:date_of_birth"`
	EmailAddress           *string    `gorm:"column:email_address"`
	MobilePhone            *string    `gorm:"column:mobile_phone"`
	AlternativeMobilePhone *string    `gorm:"column:alternative_mobile_phone"`
	BankName               *string    `gorm:"column:bank_name"`
	FullHomeAddress        *string    `gorm:"column:full_home_address"`
	PassportOnBVN          *string    `gorm:"column:passport_on_bvn"`
	City                   *string    `gorm:"column:city"`
	Landmark               *string    `gorm:"column:landmark"`
}

func NewInternalRepository(db *gorm.DB) *InternalRepository {
	return &InternalRepository{db: db}
}

func (r *InternalRepository) logInternalCallbackQuery(name string, startedAt time.Time, detail string, err error) {
	duration := time.Since(startedAt)
	if err == nil && duration < internalCallbackSlowQueryThreshold {
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) && duration < internalCallbackSlowQueryThreshold {
		return
	}

	message := fmt.Sprintf("loanproduct internal callback query=%s duration=%s", name, duration.Round(time.Millisecond))
	if detail != "" {
		message += " " + detail
	}

	if sqlDB, dbErr := r.db.DB(); dbErr == nil {
		stats := sqlDB.Stats()
		message += fmt.Sprintf(
			" db_open=%d db_in_use=%d db_idle=%d db_wait_count=%d db_wait_duration=%s",
			stats.OpenConnections,
			stats.InUse,
			stats.Idle,
			stats.WaitCount,
			stats.WaitDuration.Round(time.Millisecond),
		)
	}

	if err != nil {
		log.Printf("%s err=%v", message, err)
		return
	}

	log.Print(message)
}

func (r *InternalRepository) WithTx(ctx context.Context, fn func(*InternalRepository) error) error {
	startedAt := time.Now()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewInternalRepository(tx))
	})
	r.logInternalCallbackQuery("WithTx", startedAt, "", err)
	return err
}

func (r *InternalRepository) GetApplicationByRefForUpdate(ctx context.Context, ref string) (*LoanApplication, error) {
	var row LoanApplication
	startedAt := time.Now()
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("application_ref = ?", ref).
		First(&row).Error
	r.logInternalCallbackQuery("GetApplicationByRefForUpdate", startedAt, fmt.Sprintf("application_ref=%s", ref), err)
	return &row, err
}

func (r *InternalRepository) InsertStatusEvent(ctx context.Context, ev *LoanApplicationStatusEvent) (bool, error) {
	startedAt := time.Now()
	tx := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).
		Create(ev)
	r.logInternalCallbackQuery("InsertStatusEvent", startedAt, fmt.Sprintf("rows_affected=%d", tx.RowsAffected), tx.Error)
	return tx.RowsAffected == 1, tx.Error
}

func (r *InternalRepository) ListLoanApplicationsForCBA(ctx context.Context) ([]cbaApplicationReadRow, error) {
	var rows []cbaApplicationReadRow

	startedAt := time.Now()
	err := r.db.WithContext(ctx).
		Table("wallet_loan_applications").
		Select(`
			wallet_loan_applications.application_ref,
			wallet_loan_applications.mobile_user_id,
			wallet_loan_applications.core_customer_id AS application_core_customer_id,
			wallet_users.core_customer_id AS user_core_customer_id,
			wallet_loan_applications.phone_number,
			wallet_loan_applications.loan_product_type,
			wallet_loan_applications.business_address,
			wallet_loan_applications.business_value,
			wallet_loan_applications.business_type,
			wallet_loan_applications.requested_amount,
			wallet_loan_applications.loan_status,
			wallet_loan_applications.tenure,
			wallet_loan_applications.tenure_value,
			wallet_bvn_records.id AS bvn_record_id,
			wallet_bvn_records.bvn,
			wallet_bvn_records.first_name,
			wallet_bvn_records.middle_name,
			wallet_bvn_records.last_name,
			wallet_bvn_records.gender,
			wallet_bvn_records.nationality,
			wallet_bvn_records.state_of_origin,
			wallet_bvn_records.date_of_birth,
			wallet_bvn_records.email_address,
			wallet_bvn_records.mobile_phone,
			wallet_bvn_records.alternative_mobile_phone,
			wallet_bvn_records.bank_name,
			wallet_bvn_records.full_home_address,
			wallet_bvn_records.passport_on_bvn,
			wallet_bvn_records.city,
			wallet_bvn_records.landmark
		`).
		Joins("LEFT JOIN wallet_users ON wallet_users.id = wallet_loan_applications.mobile_user_id").
		Joins("LEFT JOIN wallet_bvn_records ON wallet_bvn_records.bvn = wallet_users.bvn").
		Order("wallet_loan_applications.created_at DESC").
		Find(&rows).Error
	r.logInternalCallbackQuery("ListLoanApplicationsForCBA", startedAt, fmt.Sprintf("rows=%d", len(rows)), err)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *InternalRepository) GetMostRecentEmbryoLoanApplicationForCBA(ctx context.Context, mobileUserID string) (*cbaApplicationReadRow, error) {
	var row cbaApplicationReadRow

	startedAt := time.Now()
	err := r.db.WithContext(ctx).
		Table("wallet_loan_applications").
		Select(cbaApplicationSelectColumns).
		Joins("LEFT JOIN wallet_users ON wallet_users.id = wallet_loan_applications.mobile_user_id").
		Joins("LEFT JOIN wallet_bvn_records ON wallet_bvn_records.bvn = wallet_users.bvn").
		Where("wallet_loan_applications.mobile_user_id = ?", mobileUserID).
		Where(
			"(wallet_loan_applications.loan_status = ? OR wallet_users.customer_status = ?)",
			LoanStatusEmbryo,
			models.CustomerStatusEmbryo,
		).
		Order("wallet_loan_applications.created_at DESC").
		Take(&row).Error
	r.logInternalCallbackQuery("GetMostRecentEmbryoLoanApplicationForCBA", startedAt, fmt.Sprintf("mobile_user_id=%s", mobileUserID), err)
	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (r *InternalRepository) GetLoanApplicationForCBAByRef(ctx context.Context, applicationRef string) (*cbaApplicationReadRow, error) {
	var row cbaApplicationReadRow

	startedAt := time.Now()
	err := r.db.WithContext(ctx).
		Table("wallet_loan_applications").
		Select(cbaApplicationSelectColumns).
		Joins("LEFT JOIN wallet_users ON wallet_users.id = wallet_loan_applications.mobile_user_id").
		Joins("LEFT JOIN wallet_bvn_records ON wallet_bvn_records.bvn = wallet_users.bvn").
		Where("wallet_loan_applications.application_ref = ?", applicationRef).
		Order("wallet_loan_applications.created_at DESC").
		Take(&row).Error
	r.logInternalCallbackQuery("GetLoanApplicationForCBAByRef", startedAt, fmt.Sprintf("application_ref=%s", applicationRef), err)
	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (r *InternalRepository) ListEmbryoLoanApplicationSummariesForCBA(ctx context.Context, limit, offset int) ([]cbaEmbryoApplicationSummaryRow, int64, error) {
	var rows []cbaEmbryoApplicationSummaryRow
	var total int64

	countStartedAt := time.Now()
	if err := r.embryoLoanApplicationSummariesBaseQuery(ctx).Count(&total).Error; err != nil {
		r.logInternalCallbackQuery("CountEmbryoLoanApplicationSummariesForCBA", countStartedAt, fmt.Sprintf("limit=%d offset=%d", limit, offset), err)
		return nil, 0, err
	}
	r.logInternalCallbackQuery("CountEmbryoLoanApplicationSummariesForCBA", countStartedAt, fmt.Sprintf("limit=%d offset=%d total=%d", limit, offset, total), nil)
	if total == 0 {
		return []cbaEmbryoApplicationSummaryRow{}, 0, nil
	}

	listStartedAt := time.Now()
	err := r.embryoLoanApplicationSummariesBaseQuery(ctx).
		Joins("LEFT JOIN wallet_bvn_records ON wallet_bvn_records.bvn = wallet_users.bvn").
		Select(`
			wallet_loan_applications.application_ref,
			wallet_loan_applications.mobile_user_id,
			wallet_loan_applications.phone_number,
			wallet_bvn_records.first_name,
			wallet_bvn_records.middle_name,
			wallet_bvn_records.last_name,
			wallet_bvn_records.gender,
			wallet_loan_applications.loan_status,
			wallet_users.customer_status
		`).
		Limit(limit).
		Offset(offset).
		Order("wallet_loan_applications.created_at DESC").
		Find(&rows).Error
	r.logInternalCallbackQuery("ListEmbryoLoanApplicationSummariesForCBA", listStartedAt, fmt.Sprintf("limit=%d offset=%d rows=%d", limit, offset, len(rows)), err)
	if err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

func (r *InternalRepository) embryoLoanApplicationSummariesBaseQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("wallet_loan_applications").
		Joins("LEFT JOIN wallet_users ON wallet_users.id = wallet_loan_applications.mobile_user_id").
		Where(
			"(wallet_loan_applications.loan_status = ? OR wallet_users.customer_status = ?)",
			LoanStatusEmbryo,
			models.CustomerStatusEmbryo,
		)
}

func (r *InternalRepository) GetLoanApplicationBVNRecordForCBA(ctx context.Context, mobileUserID string) (*cbaBVNRecordReadRow, error) {
	var row cbaBVNRecordReadRow

	startedAt := time.Now()
	err := r.db.WithContext(ctx).
		Table("wallet_loan_applications").
		Select(`
			wallet_loan_applications.application_ref,
			wallet_bvn_records.bvn,
			wallet_bvn_records.first_name,
			wallet_bvn_records.middle_name,
			wallet_bvn_records.last_name,
			wallet_bvn_records.gender,
			wallet_bvn_records.nationality,
			wallet_bvn_records.state_of_origin,
			wallet_bvn_records.date_of_birth,
			wallet_bvn_records.email_address,
			wallet_bvn_records.mobile_phone,
			wallet_bvn_records.alternative_mobile_phone,
			wallet_bvn_records.bank_name,
			wallet_bvn_records.full_home_address,
			wallet_bvn_records.passport_on_bvn,
			wallet_bvn_records.city,
			wallet_bvn_records.landmark
		`).
		Joins("INNER JOIN wallet_users ON wallet_users.id = wallet_loan_applications.mobile_user_id").
		Joins("INNER JOIN wallet_bvn_records ON wallet_bvn_records.bvn = wallet_users.bvn").
		Where("wallet_loan_applications.mobile_user_id = ?", mobileUserID).
		Order("wallet_loan_applications.created_at DESC").
		Take(&row).Error
	r.logInternalCallbackQuery("GetLoanApplicationBVNRecordForCBA", startedAt, fmt.Sprintf("mobile_user_id=%s", mobileUserID), err)
	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (r *InternalRepository) GetUserByCoreCustomerIDForUpdate(ctx context.Context, coreCustomerID string) (*models.User, error) {
	var row models.User
	startedAt := time.Now()
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("core_customer_id = ?", coreCustomerID).
		First(&row).Error
	r.logInternalCallbackQuery("GetUserByCoreCustomerIDForUpdate", startedAt, fmt.Sprintf("core_customer_id=%s", coreCustomerID), err)
	return &row, err
}

func (r *InternalRepository) InsertCustomerStatusEvent(ctx context.Context, ev *CustomerStatusEvent) (bool, error) {
	startedAt := time.Now()
	tx := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).
		Create(ev)
	r.logInternalCallbackQuery("InsertCustomerStatusEvent", startedAt, fmt.Sprintf("rows_affected=%d", tx.RowsAffected), tx.Error)
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

	startedAt := time.Now()
	tx := r.db.WithContext(ctx).
		Model(&LoanApplication{}).
		Where("application_ref = ?", ref).
		Updates(updates)
	r.logInternalCallbackQuery("UpdateApplicationStatus", startedAt, fmt.Sprintf("application_ref=%s rows_affected=%d", ref, tx.RowsAffected), tx.Error)
	return tx.Error
}

func (r *InternalRepository) UpdateUserCustomerStatus(ctx context.Context, coreCustomerID string, status models.CustomerStatus) error {
	startedAt := time.Now()
	tx := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("core_customer_id = ?", coreCustomerID).
		Update("customer_status", status)
	r.logInternalCallbackQuery("UpdateUserCustomerStatus", startedAt, fmt.Sprintf("core_customer_id=%s rows_affected=%d", coreCustomerID, tx.RowsAffected), tx.Error)
	return tx.Error
}

func (r *InternalRepository) LinkWalletUserCoreCustomerIDByBVN(ctx context.Context, bvn, coreCustomerID string) (int64, error) {
	startedAt := time.Now()
	tx := r.db.WithContext(ctx).
		Table("wallet_users").
		Where("bvn = ?", bvn).
		Update("core_customer_id", coreCustomerID)
	r.logInternalCallbackQuery("LinkWalletUserCoreCustomerIDByBVN", startedAt, fmt.Sprintf("core_customer_id=%s rows_affected=%d", coreCustomerID, tx.RowsAffected), tx.Error)
	if tx.Error != nil {
		return 0, tx.Error
	}

	return tx.RowsAffected, nil
}
