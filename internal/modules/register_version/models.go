package registerversion

import (
	"time"

	"gorm.io/gorm"
)

const RegisterVersionKey = "register_version"

type SystemPreference struct {
	ID              int64      `gorm:"type:bigint;primaryKey" json:"id"`
	PreferenceKey   string     `gorm:"column:preference_key;type:text;uniqueIndex" json:"preference_key"`
	PreferenceValue string     `gorm:"column:preference_value;type:text;not null" json:"preference_value"`
	UpdatedBy       string     `gorm:"column:updated_by;type:text;not null;default:'system'" json:"updated_by"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:timestamptz;autoCreateTime" json:"created_at"`
	UpdatedAt       *time.Time `gorm:"column:updated_at;type:timestamptz;autoUpdateTime" json:"updated_at"`
}

func (s *SystemPreference) BeforeCreate(*gorm.DB) error {
	if s.UpdatedBy == "" {
		s.UpdatedBy = "system"
	}
	return nil
}

func (SystemPreference) TableName() string {
	return "system_preferences"
}
