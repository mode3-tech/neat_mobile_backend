package referrals

import (
	"context"
	"neat_mobile_app_backend/internal/modules/transaction"
	"time"
)


const ReferralCashbackAmountNaira int64 = 10


const ReferralCashbackAmountKobo = ReferralCashbackAmountNaira * 100

const CashbackSourceReferral = "referral"


type TransactionService interface {
	AddTransaction(ctx context.Context, txRow *transaction.Transaction) error
}

type RedeemedReferral struct {
	ID           string    `gorm:"column:id"`
	ReferrerName string    `gorm:"column:referrer_name"`
	ReferredName string    `gorm:"column:referred_name"`
	RedeemedAt   time.Time `gorm:"column:redeemed_at"`
}
