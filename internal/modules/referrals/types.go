package referrals

import "time"

type RedeemedReferral struct {
	ID           string    `gorm:"column:id"`
	ReferrerName string    `gorm:"column:referrer_name"`
	ReferredName string    `gorm:"column:referred_name"`
	RedeemedAt   time.Time `gorm:"column:redeemed_at"`
}
