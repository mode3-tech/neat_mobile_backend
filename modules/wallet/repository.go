package wallet

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
