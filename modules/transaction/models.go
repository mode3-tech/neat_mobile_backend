package transaction

import (
	"neat_mobile_app_backend/internal/types"
	"time"
)

type Transaction struct {
	ID                  string              `gorm:"primaryKey;type:text"`
	MobileUserID        string              `gorm:"type:text;not null;index"`
	WalletID            string              `gorm:"type:text;not null;index"`
	Type                TransactionType     `gorm:"type:text;not null"` // "credit" | "debit"
	Category            TransactionCategory `gorm:"column:transaction_category;not null"`
	Amount              int64               `gorm:"type:bigint;not null"`
	Charges             int64               `gorm:"type:bigint;not null;default:0"`
	VAT                 int64               `gorm:"bigint;not null;default:0"`
	BalanceBefore       int64               `gorm:"type:bigint;not null"` // snapshot at time of tx
	BalanceAfter        int64               `gorm:"type:bigint;not null"` // snapshot at time of tx
	Reference           string              `gorm:"type:text;not null"`   // internal reference
	ProviderReference   string              `gorm:"type:text;index"`      // Providus ref — idempotency key
	SessionID           string              `gorm:"column:session_id"`
	Narration           *string             `gorm:"type:text"`
	Description         string              `gorm:"column:description;type:text"`
	CounterpartyName    string              `gorm:"type:text"`
	CounterpartyAccount string              `gorm:"type:text"`
	CounterpartyBank    string              `gorm:"type:text"`
	Status              TransactionStatus   `gorm:"type:text;not null"` // "pending"|"successful"|"failed"|"reversed"
	Source              string              `gorm:"type:text;not null"` // "transfer"|"credit"|"loan_disbursement"|"loan_repayment" etc.
	Metadata            types.JSONMap       `gorm:"type:jsonb"`
	CreatedAt           time.Time           `gorm:"autoCreateTime"`
	UpdatedAt           *time.Time          `gorm:"autoUpdateTime"`
}

func (Transaction) TableName() string {
	return "wallet_transactions"
}
