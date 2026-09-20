package tier

import (
	"context"

	"gorm.io/gorm"
)

var tierRequirements = map[int][]string{
	1: {"name", "dob", "phone", "address", "bvn"},
	2: {"bvn", "nin", "passport_photo"},
	3: {"bvn", "nin", "passport_photo", "utility_bill", "address verification"},
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetTiers(ctx context.Context) ([]WalletAccountTier, error) {
	var tiers []WalletAccountTier
	if err := r.db.WithContext(ctx).Find(&tiers).Error; err != nil {
		return nil, err
	}
	return tiers, nil
}

func (r *Repository) GetLatestSubmissions(ctx context.Context, mobileUserID string) (map[TierRequirement]*TierSubmission, error) {
	var rows []TierSubmission
	if err := r.db.WithContext(ctx).Raw(`
			SELECT DISTINCT ON (requirement) *
			FROM wallet_tier_submissions
			WHERE user_id = ?
			ORDER BY requirement, created_at DESC
		`, mobileUserID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[TierRequirement]*TierSubmission)
	for i := range rows {
		out[rows[i].Requirement] = &rows[i]
	}
	return out, nil
}
