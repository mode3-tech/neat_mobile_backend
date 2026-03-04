package verification

import (
	"context"
	"neat_mobile_app_backend/models"

	"gorm.io/gorm"
)

type VerificationRepo struct {
	db *gorm.DB
}

func NewVerification(db *gorm.DB) *VerificationRepo {
	return &VerificationRepo{db: db}
}

func (r *VerificationRepo) AddVerification(ctx context.Context, verification *models.VerificationRecord) error {
	return r.db.WithContext(ctx).Create(verification).Error
}
