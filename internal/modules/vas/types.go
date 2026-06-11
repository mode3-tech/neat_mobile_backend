package vas

import (
	"neat_mobile_app_backend/internal/types"
	"time"
)

type AccountType string

const (
	AccountTypePrepaid  AccountType = "prepaid"
	AccountTypePostpaid AccountType = "postpaid"
)

type TransactionStatus string

const (
	TransactionStatusPending         TransactionStatus = "pending"
	TransactionStatusSuccessful      TransactionStatus = "successful"
	TransactionStatusFailed          TransactionStatus = "failed"
	TransactionStatusReversed        TransactionStatus = "reversed"
	TransactionStatusReversalPending TransactionStatus = "reversal_pending"
)

type TransactionType string

const (
	TransactionTypeDebit  TransactionType = "debit"
	TransactionTypeCredit TransactionType = "credit"
)

type TransactionSource string

const (
	TransactionSourceDebit            TransactionSource = "debit"
	TransactionSourceCredit           TransactionSource = "credit"
	TransactionSourceLoanDisbursement TransactionSource = "loan_disbursement"
	TransactionSourceAutoSave         TransactionSource = "auto_save"
	TransactionSourceLoanRepayment    TransactionSource = "loan_repayment"
	TransactionSourceAutoRepayment    TransactionSource = "auto_repayment"
	TransactionSourceCard             TransactionSource = "card"
)

type TransactionCategory string

const (
	TransactionCategoryTransferFrom  TransactionCategory = "transfer_from"
	TransactionCategoryTransferTo    TransactionCategory = "transfer_to"
	TransactionCategoryAirtime       TransactionCategory = "airtime"
	TransactionCategoryMobileData    TransactionCategory = "mobile_data"
	TransactionCategoryReversal      TransactionCategory = "reversal"
	TransactionCategoryTV            TransactionCategory = "tv"
	TransactionCategoryElectricity   TransactionCategory = "electricity"
	TransactionCategoryCardPayment   TransactionCategory = "card_payment"
	TransactionCategoryLoanRepayment TransactionCategory = "loan_repayment"
)

var TransactionCategories = map[TransactionCategory]string{
	TransactionCategoryTransferFrom:  "Transfer From",
	TransactionCategoryTransferTo:    "Transfer To",
	TransactionCategoryAirtime:       "Airtime",
	TransactionCategoryMobileData:    "Mobile Data",
	TransactionCategoryReversal:      "Reversal",
	TransactionCategoryTV:            "TV",
	TransactionCategoryElectricity:   "Electricity",
	TransactionCategoryCardPayment:   "Card Payment",
	TransactionCategoryLoanRepayment: "Loan Repayment",
}

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
	Source              TransactionSource   `gorm:"type:text;not null"` // "transfer"|"credit"|"loan_disbursement"|"loan_repayment" etc.
	Metadata            types.JSONMap       `gorm:"type:jsonb"`
	CreatedAt           time.Time           `gorm:"autoCreateTime"`
	UpdatedAt           *time.Time          `gorm:"autoUpdateTime"`
}

func (Transaction) TableName() string { return "wallet_transactions" }

type CustomerWallet struct {
	ID               string        `gorm:"column:id;type:text;primaryKey;index"`
	InternalWalletID string        `gorm:"column:internal_wallet_id;type:text;not null;uniqueIndex"`
	MobileUserID     string        `gorm:"column:mobile_user_id;type:text;not null;index"`
	PhoneNumber      string        `gorm:"column:phone_number;type:text;not null"`
	WalletCustomerID string        `gorm:"column:wallet_customer_id;type:text;not null"`
	Metadata         types.JSONMap `gorm:"column:metadata;type:jsonb;not null"`
	BVN              string        `gorm:"column:bvn;type:text;not null"`
	Currency         string        `gorm:"column:currency;type:text;not null"`
	DateOfBirth      string        `gorm:"column:date_of_birth;type:text;not null"`
	FirstName        string        `gorm:"column:first_name;type:text;not null"`
	LastName         string        `gorm:"column:last_name;type:text;not null"`
	Email            string        `gorm:"column:email;type:text;not null"`
	Address          string        `gorm:"column:address;type:text;not null"`
	MerchantID       string        `gorm:"column:merchant_id;type:text;not null"`
	Tier             string        `gorm:"column:tier;type:text;not null"`
	WalletID         string        `gorm:"column:wallet_id;type:text;not null"`
	Mode             string        `gorm:"column:mode;type:text;not null"`
	BankName         string        `gorm:"column:bank_name;type:text;not null"`
	BankCode         string        `gorm:"column:bank_code;type:text;not null"`
	AccountNumber    string        `gorm:"column:account_number;type:text;not null"`
	AccountName      string        `gorm:"column:account_name;type:text;not null"`
	AccountRef       string        `gorm:"column:account_ref;type:text;not null"`
	BookedBalance    int64         `gorm:"column:booked_balance;type:bigint;not null;default:0"`
	AvailableBalance int64         `gorm:"column:available_balance;type:bigint;not null;default:0"`
	Status           string        `gorm:"column:status;type:text;not null"`
	WalletType       string        `gorm:"column:wallet_type;type:text;not null"`
	Updated          bool          `gorm:"column:updated;type:boolean;not null;default:false"`
	CreatedAt        time.Time     `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt        *time.Time    `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (CustomerWallet) TableName() string { return "wallet_customer_wallets" }
