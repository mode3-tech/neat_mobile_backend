package registerv2

import "time"

// ProviderPreference is a singleton row (id is always 1) recording which BaaS
// provider new registrations should use. Switching it only affects new
// registrations going forward - existing wallets stay on whichever provider
// created them.
type ProviderPreference struct {
	ID        int16     `gorm:"column:id;primaryKey"`
	Provider  string    `gorm:"column:provider;type:text;not null"`
	UpdatedBy *string   `gorm:"column:updated_by;type:text"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (ProviderPreference) TableName() string { return "wallet_provider_preference" }
