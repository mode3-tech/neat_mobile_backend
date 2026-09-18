package appversion

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

func (r *Repository) GetAppVersionInfo(ctx context.Context, appOS string) (*AppVersionInfo, error) {
	var info AppVersionInfo
	err := r.db.Where("app_os = ?", appOS).First(&info).Error
	return &info, err
}
