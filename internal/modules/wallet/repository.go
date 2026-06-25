package wallet

import (
	"context"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/modules/transaction"
	"neat_mobile_app_backend/models"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByMobileUserID(ctx context.Context, mobileUserID string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("id = ?", mobileUserID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateWallet(ctx context.Context, wallet *CustomerWallet) error {
	if err := r.db.WithContext(ctx).Create(wallet).Error; err != nil {
		return err
	}
	return nil
}

func (r *Repository) GetWallet(ctx context.Context, mobileUserID string) (*CustomerWallet, error) {
	var wallet CustomerWallet
	err := r.db.WithContext(ctx).Where("mobile_user_id = ?", mobileUserID).First(&wallet).Error
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

		var wallet CustomerWallet
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("internal_wallet_id = ?", walletID).
			First(&wallet).Error; err != nil {
			return err
		}

		if err := tx.Model(&transaction.Transaction{}).
			Where("id = ?", txID).
			Updates(map[string]interface{}{
				"provider_reference": providerRef,
				"status":             status,
				"balance_before":     wallet.AvailableBalance,
				"balance_after":      wallet.AvailableBalance - totalDebit,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&CustomerWallet{}).
			Where("internal_wallet_id = ?", walletID).
			Updates(map[string]interface{}{
				"booked_balance":    gorm.Expr("booked_balance - ?", totalDebit),
				"available_balance": gorm.Expr("available_balance - ?", totalDebit),
				"updated_at":        time.Now(),
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

func (r *Repository) CreditWalletAtomically(ctx context.Context, tx *transaction.Transaction, amount int64) error {
	return r.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {

		var wallet CustomerWallet
		if err := db.Set("gorm:query_option", "FOR UPDATE").
			Where("wallet_id = ?", tx.WalletID).
			First(&wallet).Error; err != nil {
			return err
		}

		tx.BalanceBefore = wallet.AvailableBalance
		tx.BalanceAfter = wallet.AvailableBalance + amount

		if err := db.Create(tx).Error; err != nil {
			return err
		}

		return db.Model(&CustomerWallet{}).
			Where("wallet_id = ?", tx.WalletID).
			Updates(map[string]interface{}{
				"booked_balance":    gorm.Expr("booked_balance + ?", amount),
				"available_balance": gorm.Expr("available_balance + ?", amount),
				"updated_at":        time.Now(),
			}).Error
	})
}

func (r *Repository) CreateExpectedDeposit(ctx context.Context, expectedDeposit *ExpectedDeposit) error {
	return r.db.WithContext(ctx).Create(expectedDeposit).Error
}

func (r *Repository) SumTransactionsInWindow(ctx context.Context, mobileUserID string, txType transaction.TransactionType, from, to time.Time) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&transaction.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("mobile_user_id = ? AND type = ? AND created_at >= ? AND created_at < ? AND status = ?",
			mobileUserID, txType, from, to, transaction.TransactionStatusSuccessful).
		Scan(&total).Error
	return total, err
}

func (r *Repository) FindTransactionByProviderRef(ctx context.Context, providerRef string) (*transaction.Transaction, error) {
	var tx transaction.Transaction
	err := r.db.WithContext(ctx).
		Where("provider_reference = ?", providerRef).
		First(&tx).Error
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *Repository) ReverseDebitTransaction(ctx context.Context, txID, walletID string) error {
	return r.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		var tx transaction.Transaction
		if err := db.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", txID).
			First(&tx).Error; err != nil {
			return err
		}

		if tx.Status != transaction.TransactionStatusPending {
			return nil // already finalized
		}

		var wallet CustomerWallet
		if err := db.Set("gorm:query_option", "FOR UPDATE").
			Where("internal_wallet_id = ?", walletID).
			First(&wallet).Error; err != nil {
			return err
		}

		if err := db.Model(&transaction.Transaction{}).
			Where("id = ?", txID).
			Updates(map[string]interface{}{
				"status":        transaction.TransactionStatusFailed,
				"balance_after": wallet.AvailableBalance,
			}).Error; err != nil {
			return err
		}

		return db.Model(&CustomerWallet{}).
			Where("internal_wallet_id = ?", walletID).
			Updates(map[string]interface{}{
				"booked_balance":    gorm.Expr("booked_balance + ?", tx.Amount),
				"available_balance": gorm.Expr("available_balance + ?", tx.Amount),
				"updated_at":        time.Now(),
			}).Error
	})
}

func (r *Repository) FindUserByWalletCustomerID(ctx context.Context, walletCustomerID string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Table("wallet_users").
		Joins("JOIN wallet_customer_wallets ON wallet_customer_wallets.mobile_user_id = wallet_users.id").
		Where("wallet_customer_wallets.wallet_customer_id = ?", walletCustomerID).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
