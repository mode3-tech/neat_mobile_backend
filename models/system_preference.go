package models

type SystemPreference struct {
	PreferenceKey   string `gorm:"column:preference_key;type:text;primaryKey"`
	PreferenceValue string `gorm:"column:preference_value;type:text;not null"`
}

func (SystemPreference) TableName() string {
	return "system_preferences"
}
