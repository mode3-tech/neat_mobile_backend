package models

import "time"

const (
	CashbackEntryCredit   = "credit"
	CashbackEntryDebit    = "debit"
	CashbackEntryReversal = "reversal"
)

type Cashback struct {
	ID             string    `gorm:"column:id;type:text;primaryKey"`
	MobileUserID   string    `gorm:"column:mobile_user_id;not null"`
	CashbackBefore int64     `gorm:"column:cashback_before;type:bigint"`
	CashbackAfter  int64     `gorm:"column:cashback_after;type:bigint"`
	CashbackAmount int64     `gorm:"column:cashback_amount;type:bigint;not null;default:0"`
	Source         string    `gorm:"column:cashback_source;type:text"` //e.g referral code, vas
	EntryType      string    `gorm:"column:entry_type;type:text;not null;default:'credit'"`
	TransactionID  *string   `gorm:"column:transaction_id;type:text;index"`
	CreatedAt      time.Time `gorm:"column:created_at;type:timestamptz;autoCreateTime"`
}

func (Cashback) TableName() string {
	return "wallet_cashbacks"
}
