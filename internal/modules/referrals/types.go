package referrals

import (
	"time"
)

const ReferralCashbackAmountNaira int64 = 10

const ReferralCashbackAmountKobo = ReferralCashbackAmountNaira * 100

const CashbackSourceReferral = "referral"
const CashbackSourceVAS = "vas"

type RedeemedReferral struct {
	ID           string    `gorm:"column:id"`
	ReferrerName string    `gorm:"column:referrer_name"`
	ReferredName string    `gorm:"column:referred_name"`
	RedeemedAt   time.Time `gorm:"column:redeemed_at"`
}
