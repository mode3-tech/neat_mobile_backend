package smsbilling

import "time"

type SMSChargeStatus string

const (
	SMSChargeStatusPending    SMSChargeStatus = "pending"
	SMSChargeStatusSuccessful SMSChargeStatus = "successful"
	// SMSChargeStatusAbandoned is a terminal, non-retried state reached once a
	// charge has been attempted maxAttempts times without success - keeps the
	// row (and its audit trail) instead of deleting it, but stops the sweep
	// from touching it forever.
	SMSChargeStatusAbandoned SMSChargeStatus = "abandoned"
)

// SMSCharge is a billing obligation for one billable outgoing SMS, created
// immediately after the provider accepts the message. Price and unit count
// are snapshotted at creation so a later tariff change never alters
// historical debt. Collection against the customer's wallet is a separate,
// retryable step (see Repository.AttemptCollect) - Reference is the
// idempotency key carried onto the resulting wallet_transactions row so a
// charge can never be collected twice.
type SMSCharge struct {
	ID            string          `gorm:"column:id;type:text;primaryKey"`
	MobileUserID  string          `gorm:"column:mobile_user_id;type:text;not null;index"`
	Phone         string          `gorm:"column:phone;type:text;not null"`
	Purpose       string          `gorm:"column:purpose;type:text;not null"`
	Units         int             `gorm:"column:units;type:int;not null;default:1"`
	UnitPriceKobo int64           `gorm:"column:unit_price_kobo;type:bigint;not null"`
	AmountKobo    int64           `gorm:"column:amount_kobo;type:bigint;not null"`
	Status        SMSChargeStatus `gorm:"column:status;type:text;not null;default:pending;check:status IN ('pending','successful','abandoned')"`
	TransactionID *string         `gorm:"column:transaction_id;type:text"`
	Reference     string          `gorm:"column:reference;type:text;not null;uniqueIndex"`
	Attempts      int             `gorm:"column:attempts;type:int;not null;default:0"`
	LastAttemptAt *time.Time      `gorm:"column:last_attempt_at;type:timestamptz"`
	CreatedAt     time.Time       `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt     *time.Time      `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (SMSCharge) TableName() string {
	return "wallet_sms_charges"
}

// customerWallet mirrors the columns of wallet.CustomerWallet needed here.
// Redeclared locally rather than imported - this codebase's established
// convention for a module that needs to read/lock the shared
// wallet_customer_wallets table without depending on the wallet package (see
// vas.CustomerWallet for the same pattern). Must never be registered in
// AutoMigrate: wallet.CustomerWallet already owns that table's schema.
type customerWallet struct {
	InternalWalletID string `gorm:"column:internal_wallet_id;primaryKey"`
	MobileUserID     string `gorm:"column:mobile_user_id"`
	WalletCustomerID string `gorm:"column:wallet_customer_id"`
	AvailableBalance int64  `gorm:"column:available_balance"`
	BookedBalance    int64  `gorm:"column:booked_balance"`
}

func (customerWallet) TableName() string {
	return "wallet_customer_wallets"
}

// ledgerTransaction mirrors the columns of transaction.Transaction needed to
// record a collected SMS charge as a normal wallet debit. Same rationale as
// customerWallet above - not registered in AutoMigrate.
type ledgerTransaction struct {
	ID            string    `gorm:"column:id;type:text;primaryKey"`
	MobileUserID  string    `gorm:"column:mobile_user_id;type:text"`
	WalletID      string    `gorm:"column:wallet_id;type:text"`
	Type          string    `gorm:"column:type;type:text"`
	Category      string    `gorm:"column:transaction_category;type:text"`
	Amount        int64     `gorm:"column:amount;type:bigint"`
	BalanceBefore int64     `gorm:"column:balance_before;type:bigint"`
	BalanceAfter  int64     `gorm:"column:balance_after;type:bigint"`
	Reference     string    `gorm:"column:reference;type:text"`
	Description   string    `gorm:"column:description;type:text"`
	Status        string    `gorm:"column:status;type:text"`
	Source        string    `gorm:"column:source;type:text"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamptz"`
}

func (ledgerTransaction) TableName() string {
	return "wallet_transactions"
}
