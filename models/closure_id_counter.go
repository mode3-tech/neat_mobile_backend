package models

import "time"

type ClosureReferenceCounter struct {
	Period       string     `gorm:"primaryKey"`
	LastSequence int64      `gorm:"column:last_sequence;not null;type:bigint"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime;type:timestamptz;not null"`
	UpdatedAt    *time.Time `gorm:"column:updated_at;autoUpdateTime;type:timestamptz"`
}

func (ClosureReferenceCounter) TableName() string {
	return "wallet_account_closure_reference_counters"
}
