package loanproduct

import (
	"context"
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
	Phone           string     `gorm:"column:phone"`
	DOB             *time.Time `gorm:"column:dob"`
	BVN             string     `gorm:"column:bvn"`
	NIN             string     `gorm:"column:nin"`
	IsPhoneVerified bool       `gorm:"column:is_phone_verified"`
	IsBVNVerified   bool       `gorm:"column:is_bvn_verified"`
	IsNINVerified   bool       `gorm:"column:is_nin_verified"`
}

func (r *Repository) GetUser(ctx context.Context, userID string) (*row, error) {
	var row row

	if err := r.db.WithContext(ctx).Table("wallet_users").Select("phone, dob, bvn, nin, is_phone_verified, is_bvn_verified, is_nin_verified").Where("id = ? ", userID).First(&row).Error; err != nil {
		return nil, err
	}

	return &row, nil
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
