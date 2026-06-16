package user

import (
	"context"
	"neat_mobile_app_backend/models"

	"gorm.io/gorm"
)

type Repository struct {
	DB *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) GetUserByUserID(ctx context.Context, mobileUserID string) (*models.User, error) {
	var user models.User
	if err := r.DB.WithContext(ctx).First(&user, "id = ?", mobileUserID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
