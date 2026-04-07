package wallet

import (
	"context"
	"neat_mobile_app_backend/models"
	"neat_mobile_app_backend/modules/device"
	"neat_mobile_app_backend/modules/transaction"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateWallet(ctx context.Context, wallet *CustomerWallet) error {
	if err := r.db.WithContext(ctx).Create(wallet).Error; err != nil {
		return err
	}
	return nil
}

func (r *Repository) GetWallet(ctx context.Context, mobileUserID, walletID string) (*CustomerWallet, error) {
	var wallet CustomerWallet
	err := r.db.WithContext(ctx).Where("mobile_user_id = ? AND internal_wallet_id = ?", mobileUserID, walletID).First(&wallet).Error
	if err != nil {
		return nil, err
	}

	return &wallet, nil
}

func (r *Repository) GetDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error) {
	var device device.UserDevice
	err := r.db.WithContext(ctx).Where("user_id = ? AND device_id = ?", mobileUserID, deviceID).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *Repository) GetUserWalletID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Select("id", "wallet_id").
		Where("id = ?", userID).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil

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

func (r *Repository) AddTransaction(ctx context.Context, transaction *transaction.Transaction) error {
	return r.db.WithContext(ctx).Create(transaction).Error
}

func (r *Repository) UpdateTransactionStatus(ctx context.Context, txID string, status transaction.TransactionStatus) error {
	return r.db.WithContext(ctx).Model(&transaction.Transaction{}).Where("id = ?", txID).Update("status", status).Error
}

func (r *Repository) UpdateTransactionProviderRef(ctx context.Context, txID, providerRef string, status transaction.TransactionStatus) error {
	return r.db.WithContext(ctx).Model(&transaction.Transaction{}).
		Where("id = ?", txID).
		Updates(map[string]interface{}{
			"provider_reference": providerRef,
			"status":             status,
		}).Error
}

func (r *Repository) CompleteDebitTransaction(ctx context.Context, txID, providerRef string, status transaction.TransactionStatus, walletID string, totalDebit int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&transaction.Transaction{}).
			Where("id = ?", txID).
			Updates(map[string]interface{}{
				"provider_reference": providerRef,
				"status":             status,
			}).Error; err != nil {
			return err
		}

		return tx.Model(&CustomerWallet{}).
			Where("wallet_id = ?", walletID).
			Updates(map[string]interface{}{
				"booked_balance":    gorm.Expr("booked_balance - ?", totalDebit),
				"available_balance": gorm.Expr("available_balance - ?", totalDebit),
			}).Error
	})
}

func (r *Repository) CreateBeneficiary(ctx context.Context, beneficiary *Beneficiary) error {
	return r.db.WithContext(ctx).Create(beneficiary).Error
}

func (r *Repository) GetBeneficiaries(ctx context.Context, mobileUserID string) ([]Beneficiary, error) {
	var beneficiaries []Beneficiary
	err := r.db.WithContext(ctx).Select("wallet_id, bank_code, account_number, account_name").Where("mobile_user_id = ?", mobileUserID).Find(&beneficiaries).Error
	return beneficiaries, err
}

func (r *Repository) GetWalletByAccountNumber(ctx context.Context, accountNumber string) (*CustomerWallet, error) {
	var w CustomerWallet
	err := r.db.WithContext(ctx).Where("account_number = ?", accountNumber).First(&w).Error
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *Repository) CreditWalletBalance(ctx context.Context, walletID string, amount int64) error {
	return r.db.WithContext(ctx).Model(&CustomerWallet{}).
		Where("wallet_id = ?", walletID).
		Updates(map[string]interface{}{
			"booked_balance":    gorm.Expr("booked_balance + ?", amount),
			"available_balance": gorm.Expr("available_balance + ?", amount),
		}).Error
}

func (r *Repository) CreateExpectedDeposit(ctx context.Context, expectedDeposit *ExpectedDeposit) error {
	return r.db.WithContext(ctx).Create(expectedDeposit).Error
}
