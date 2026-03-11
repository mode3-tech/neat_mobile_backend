package loanproduct

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

func (r *Repository) GetAllLoanProducts(ctx context.Context) ([]LoanProduct, error) {
	var products []LoanProduct
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&products).Error; err != nil {
		return nil, err
	}

	return products, nil
}
