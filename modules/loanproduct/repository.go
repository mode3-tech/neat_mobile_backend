package loanproduct

import (
	"context"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/device"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetAllLoanProducts(ctx context.Context) ([]PartialLoanProduct, error) {
	var products []PartialLoanProduct
	if err := r.db.WithContext(ctx).Model(&LoanProduct{}).Order("created_at DESC").Find(&products).Error; err != nil {
		return nil, err
	}

	return products, nil
}

func (r *Repository) GetLoanProductWithCode(ctx context.Context, code LoanType) (*LoanProduct, error) {
	var result LoanProduct
	if err := r.db.WithContext(ctx).Model(&LoanProduct{}).Where("code = ?", code).First(&result).Error; err != nil {
		return nil, err
	}

	return &result, nil
}

type row struct {
	CoreCustomerID            *string    `gorm:"column:core_customer_id"`
	Phone                     string     `gorm:"column:phone"`
	DOB                       *time.Time `gorm:"column:dob"`
	BVN                       string     `gorm:"column:bvn"`
	NIN                       string     `gorm:"column:nin"`
	IsPhoneVerified           bool       `gorm:"column:is_phone_verified"`
	IsBVNVerified             bool       `gorm:"column:is_bvn_verified"`
	IsNINVerified             bool       `gorm:"column:is_nin_verified"`
	PinHash                   string     `gorm:"column:pin_hash"`
	FailedTransactionAttempts int        `gorm:"column:failed_transaction_pin_attempts"`
	TransactionPinLockedUntil *time.Time `gorm:"column:transaction_pin_locked_until"`
}

func (r *Repository) GetUser(ctx context.Context, userID string) (*row, error) {
	var row row

	if err := r.db.WithContext(ctx).
		Table("wallet_users").
		Select("core_customer_id, phone, dob, bvn, nin, is_phone_verified, is_bvn_verified, is_nin_verified, pin_hash, failed_transaction_pin_attempts, transaction_pin_locked_until").
		Where("id = ? ", userID).
		Take(&row).Error; err != nil {
		return nil, err
	}

	return &row, nil
}

func (r *Repository) UpdateUserCoreCustomerID(ctx context.Context, userID, coreCustomerID string) error {
	tx := r.db.WithContext(ctx).
		Table("wallet_users").
		Where("id = ?", userID).
		Update("core_customer_id", coreCustomerID)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

type transactionPinStateUpdate struct {
	FailedTransactionPinAttempts int        `gorm:"column:failed_transaction_pin_attempts"`
	TransactionPinLockedUntil    *time.Time `gorm:"column:transaction_pin_locked_until"`
}

func (r *Repository) UpdateTransactionPinAttempts(ctx context.Context, userID string, attempts int, lockedUntil *time.Time) error {
	tx := r.db.WithContext(ctx).
		Table("wallet_users").
		Where("id = ?", userID).
		Select("failed_transaction_pin_attempts", "transaction_pin_locked_until").
		Updates(transactionPinStateUpdate{
			FailedTransactionPinAttempts: attempts,
			TransactionPinLockedUntil:    lockedUntil,
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *Repository) ResetTransactionPinAttempts(ctx context.Context, userID string) error {
	return r.UpdateTransactionPinAttempts(ctx, userID, 0, nil)
}

func (r *Repository) CreateEOI(ctx context.Context, eoi *LoanApplication) error {
	return r.db.WithContext(ctx).Create(eoi).Error
}

func (r *Repository) GetCoreUser(ctx context.Context, bvn string) error {
	return r.db.WithContext(ctx).Table("customer_account_info").Where("bvn = ?", bvn).Error
}

func (r *Repository) GetRuleByProductID(ctx context.Context, productID string) (*LoanProductRule, error) {
	var productRule LoanProductRule
	if err := r.db.WithContext(ctx).Model(&LoanProductRule{}).Where("product_id = ?", productID).First(&productRule).Error; err != nil {
		return nil, err
	}
	return &productRule, nil
}

func (r *Repository) GetDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error) {
	var d device.UserDevice
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND device_id = ?", mobileUserID, deviceID).
		First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) GetLoanApplicationsWithUserID(ctx context.Context, userID string) (*LoanApplication, error) {
	var loanApplication LoanApplication

	if err := r.db.WithContext(ctx).Model(&LoanApplication{}).Where("mobile_user_id = ?", userID).First(&loanApplication).Error; err != nil {
		return nil, err
	}

	return &loanApplication, nil
}

func (r *Repository) GetUserForPinVerification(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Select("id", "pin_hash", "failed_transaction_pin_attempts", "transaction_pin_locked_until").
		Where("id = ?", userID).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) IncrementFailedPinAttempts(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).
		Update("failed_transaction_pin_attempts", gorm.Expr("failed_transaction_pin_attempts + 1")).Error
}

func (r *Repository) LockTransactionPin(ctx context.Context, userID string, until time.Time) error {
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"failed_transaction_pin_attempts": 0,
			"transaction_pin_locked_until":    until,
		}).Error
}

func (r *Repository) ResetPinAttempts(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"failed_transaction_pin_attempts": 0,
			"transaction_pin_locked_until":    nil,
		}).Error
}

const loanSummaryBaseQuery = `
SELECT
    lp.name                             AS loan_product_type,
    COALESCE(l.amount, 0)               AS loan_amount,
    COALESCE(l.amount_to_be_paid, 0)    AS total_repayment,
    COALESCE(l.installment, 0)          AS periodic_repayment,
    CONCAT(l.loan_term, ' ', CASE LOWER(lp.repayment_frequency)
        WHEN 'weekly'  THEN 'Weeks'
        WHEN 'monthly' THEN 'months'
        ELSE lp.repayment_frequency
    END)                                AS loan_duration,
    COALESCE(lp.interest_rate, 0)       AS interest_rate
FROM loan_loan l
JOIN loan_loanproduct lp ON lp.id = l.product_id
`

func (r *Repository) GetLoanRepaymentSummary(ctx context.Context, loanID string) (*LoanRepayment, error) {
	var summary LoanRepayment
	err := r.db.WithContext(ctx).Raw(loanSummaryBaseQuery+" WHERE l.id = ?", loanID).Scan(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}
