package wallet

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

func (r *Repository) GetDevice(ctx context.Context, mobileUserID, deviceID string) (*device.UserDevice, error) {
	var device device.UserDevice
	err := r.db.WithContext(ctx).Where("user_id = ? AND device_id = ?", mobileUserID, deviceID).First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
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

func (r *Repository) CreateTransfer(ctx context.Context, transfer *Transfer) error {
	return r.db.WithContext(ctx).Create(transfer).Error
}

func (r *Repository) UpdateTransferStatus(ctx context.Context, transferID uint, status TransferStatus) error {
	return r.db.WithContext(ctx).Model(&Transfer{}).Where("id = ?", transferID).Update("status", status).Error
}

func (r *Repository) CreateBeneficiary(ctx context.Context, beneficiary *Beneficiary) error {
	return r.db.WithContext(ctx).Create(beneficiary).Error
}

func (r *Repository) GetBeneficiaries(ctx context.Context, mobileUserID, walletID string) ([]Beneficiary, error) {
	var beneficiaries []Beneficiary
	err := r.db.WithContext(ctx).Select("WalletID, BankCode, AccountNumber, AccountName").Where("mobile_user_id = ? AND wallet_id = ?", mobileUserID, walletID).Find(&beneficiaries).Error
	return beneficiaries, err
}
