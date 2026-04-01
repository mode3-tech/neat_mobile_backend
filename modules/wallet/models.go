package wallet

import "time"

type CustomerWallet struct {
	ID               string         `gorm:"column:id;type:text;primaryKey;index"`
	InternalWalletID string         `gorm:"column:internal_wallet_id;type:text;not null;uniqueIndex"`
	MobileUserID     string         `gorm:"column:mobile_user_id;type:text;not null;index"`
	CoreCustomerID   *string        `gorm:"column:core_customer_id;type:text"`
	PhoneNumber      string         `gorm:"column:phone_number;type:text;not null"`
	WalletCustomerID string         `gorm:"column:wallet_customer_id;type:text;not null"`
	Metadata         map[string]any `gorm:"column:metadata;type:jsonb;not null"`
	BVN              string         `gorm:"column:bvn;type:text;not null"`
	Currency         string         `gorm:"column:currency;type:text;not null"`
	DateOfBirth      string         `gorm:"column:date_of_birth;type:text;not null"`
	FirstName        string         `gorm:"column:first_name;type:text;not null"`
	LastName         string         `gorm:"column:last_name;type:text;not null"`
	Email            string         `gorm:"column:email;type:text;not null"`
	Address          string         `gorm:"column:address;type:text;not null"`
	MerchantID       string         `gorm:"column:merchant_id;type:text;not null"`
	Tier             string         `gorm:"column:tier;type:text;not null"`
	WalletID         string         `gorm:"column:wallet_id;type:text;not null"`
	Mode             string         `gorm:"column:mode;type:text;not null"`
	BankName         string         `gorm:"column:bank_name;type:text;not null"`
	BankCode         string         `gorm:"column:bank_code;type:text;not null"`
	AccountNumber    string         `gorm:"column:account_number;type:text;not null"`
	AccountName      string         `gorm:"column:account_name;type:text;not null"`
	AccountRef       string         `gorm:"column:account_ref;type:text;not null"`
	BookedBalance    int64          `gorm:"column:booked_balance;type:bigint;not null;default:0"`
	AvailableBalance int64          `gorm:"column:available_balance;type:bigint;not null;default:0"`
	Status           string         `gorm:"column:status;type:text;not null"`
	WalletType       string         `gorm:"column:wallet_type;type:text;not null"`
	Updated          bool           `gorm:"column:updated;type:boolean;not null;default:false"`
	CreatedAt        time.Time      `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt        *time.Time     `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (CustomerWallet) TableName() string {
	return "wallet_customer_wallets"
}

type Transfer struct {
	ID                   string          `gorm:"column:id;type:text;primaryKey;index"`
	MobileUserID         string          `gorm:"column:mobile_user_id;type:text;not null;index"`
	WalletID             string          `gorm:"column:wallet_id;type:text;not null;index"`
	Reference            string          `gorm:"column:reference;type:text"`
	TransactionReference string          `gorm:"column:transaction_reference;type:text"`
	SessionID            string          `gorm:"column:session_id;type:text"`
	Amount               int64           `gorm:"column:amount;type:bigint;not null"`
	Charges              int64           `gorm:"column:charges;type:bigint;not null;default:0"`
	Vat                  int64           `gorm:"column:vat;type:bigint;not null;default:0"`
	Total                int64           `gorm:"column:total;type:bigint;not null;default:0"`
	Narration            *string         `gorm:"column:narration;type:text"`
	Destination          string          `gorm:"column:destination;type:text;not null"`
	Description          string          `gorm:"column:description;type:text;not null"`
	Metadata             *map[string]any `gorm:"column:metadata;type:jsonb"`
	Status               TransferStatus  `gorm:"column:status;type:text;not null"`
	TransferType         TransferType    `gorm:"column:transfer_type;type:text;not null"`
	CreatedAt            time.Time       `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt            *time.Time      `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (Transfer) TableName() string {
	return "wallet_transfers"
}

type Beneficiary struct {
	ID            string     `gorm:"column:id;type:text;primaryKey;index"`
	MobileUserID  string     `gorm:"column:mobile_user_id;type:text;not null;index"`
	WalletID      string     `gorm:"column:wallet_id;type:text;not null;index"`
	BankCode      string     `gorm:"column:bank_code;type:text;not null"`
	AccountNumber string     `gorm:"column:account_number;type:text;not null"`
	AccountName   string     `gorm:"column:account_name;type:text;not null"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt     *time.Time `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (Beneficiary) TableName() string {
	return "wallet_beneficiaries"
}
