package card

import "time"

type Card struct {
	ID          string     `gorm:"column:id;type:text;primaryKey"`
	CardNumber  string     `gorm:"column:card_number;type:text;not null;index"`
	ReferenceID string     `gorm:"column:reference_id;type:text;not null"`
	UserID      string     `gorm:"column:user_id;type:text;not null"`
	CVV         string     `gorm:"column:cvv;type:text;not null"`
	ExpiryDate  time.Time  `gorm:"column:expiry_date;type:timestamptz;not null"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt   *time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (Card) TableName() string {
	return "wallet_cards"
}
