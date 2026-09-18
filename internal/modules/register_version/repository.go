package registerversion

import (
	"log"
	appErr "neat_mobile_app_backend/internal/errors"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetRegisterVersion() (*SystemPreference, error) {
	var preference SystemPreference
	err := r.db.Where("preference_key = ?", RegisterVersionKey).First(&preference).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("record not found")
			return nil, appErr.ErrNotFound
		}
		log.Printf("error: %v", err)
		return nil, err
	}
	return &preference, nil
}
