package referrals

import "time"

type RedeemReferralCodeRequest struct {
	ReferralCode string `json:"referral_code"`
}

type RedeemedReferralResponse struct {
	ID           string    `json:"id"`
	ReferrerName string    `json:"referrer_name"`
	ReferredName string    `json:"referred_name"`
	RedeemedAt   time.Time `json:"redeemed_at"`
}
