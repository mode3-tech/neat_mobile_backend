package loanproduct

import (
	"neat_mobile_app_backend/models"
	"time"
)

type LoanProduct struct {
	ID                    string        `gorm:"column:id;type:text;primaryKey" json:"id"`
	Code                  string        `gorm:"column:code;type:text;uniqueIndex;not null" json:"code"`
	Name                  string        `gorm:"column:name;type:text;not null"  json:"name"`
	Description           string        `gorm:"column:description;type:text;not null"  json:"description"`
	MinLoanAmount         int64         `gorm:"column:min_loan_amount;not null"  json:"min_loan_amount"`
	MaxLoanAmount         int64         `gorm:"column:max_loan_amount;not null"  json:"max_loan_amount"`
	InterestRateBPS       int           `gorm:"column:interest_rate_bps;not null"  json:"interest_rate_bps"`
	RepaymentFrequency    LoanFrequency `gorm:"column:repayment_frequency;type:text;not null"  json:"repayment_frequency"`
	GracePeriodDays       int           `gorm:"column:grace_period_days;not null;default:0" json:"grace_period_days"`
	LoanTermValue         int           `gorm:"column:loan_term_value;not null" json:"loan_term_value"`
	LatePenaltyBPS        int           `gorm:"column:late_penalty_bps;not null;default:0" json:"late_penalty_bps"`
	AllowsConcurrentLoans bool          `gorm:"column:allows_concurrent_loans;not null;default:false" json:"allows_concurrent_loans"`
	IsActive              bool          `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt             time.Time     `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime" json:"created_at"`
	UpdatedAt             *time.Time    `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime" json:"updated_at"`
}

func (LoanProduct) TableName() string {
	return "wallet_loan_products"
}

type LoanProductRule struct {
	ID                          string `gorm:"column:id;type:text;primaryKey"`
	ProductID                   string `gorm:"column:product_id;type:text;foreignKey;not null"`
	MinSavingsBalance           int64  `gorm:"column:min_savings_balance;not null"`
	MinAccountAgeDays           int    `gorm:"column:min_account_age_days;not null;default:0"`
	MaxActiveLoans              int    `gorm:"column:max_account_loans;not null;default:0"`
	RequireKYC                  *bool  `gorm:"column:require_kyc"`
	RequireBVN                  *bool  `gorm:"required_bvn"`
	RequireNIN                  *bool  `gorm:"require_bvn"`
	RequirePhoneVerified        *bool  `gorm:"require_phone_verified"`
	RequireNoOutstandingDefault *bool  `gorm:"require_no_outstanding_default"`
	HighValueThreshold          int    `gorm:"high_value_threshold;not null"`
	BranchManagerApprovalLimit  int64  `gorm:"branch_manager_approval_limit;not null"`
}

func (LoanProductRule) TableName() string {
	return "wallet_loan_product_rules"
}

type LoanProductEvaluation struct {
	ID                    string        `gorm:"column:id;type:text;primaryKey"`
	ProductID             string        `gorm:"column:product_id;type:text;foreignKey;not null;index"`
	CustomerID            string        `gorm:"column:customer_id;type:text;foreignKey;not null;index"`
	RequestedAmount       int64         `gorm:"column:requested_amount;not null"`
	LoanTermValue         int           `gorm:"column:loan_term_value"`
	LoanTermUnit          LoanFrequency `gorm:"column:loan_term_unit;not null"`
	Decision              LoanDecison   `gorm:"column:decision"`
	RequiredApprovalLevel ApprovalLevel `gorm:"column:required_approval_level"`
	FailedCodes           string        `gorm:"column:failed_codes;type:jsonb;not null;default:'[]'" json:"failed_codes"`
	ResultJSON            string        `gorm:"column:result_json;type:jsonb;not null" json:"result_json"`
	EvaluatedBy           string        `gorm:"column:evaluted_by;not null"`
	CreatedAt             time.Time     `gorm:"column:created_at;type:timestamptz;autoCreateTime;not null"`
}

func (LoanProductEvaluation) TableName() string {
	return "wallet_loan_product_evaluations"
}

type LoanApplication struct {
	ID                string        `gorm:"column:id;type:text;primaryKey"`
	MobileUserID      string        `gorm:"column:mobile_user_id;type:text;not null;index"`
	CoreCustomerID    string        `gorm:"column:core_customer_id"`
	PhoneNumber       string        `gorm:"column:phone_number;not null"`
	ApplicationRef    string        `gorm:"column:application_ref;not null;uniqueIndex"`
	CoreLoanID        *string       `gorm:"column:core_loan_id"`
	LoanProductType   LoanType      `gorm:"column:loan_product_type;not null"`
	BusinessAddress   string        `gorm:"column:business_address;not null"`
	BusinessValue     int64         `gorm:"column:business_value;not null;default:0"`
	BusinessStartDate string        `gorm:"column:business_start_date;not null"`
	BusinessType      string        `gorm:"column:business_type;not null"`
	RequestedAmount   int64         `gorm:"column:requested_amount;not null;default:0"`
	LoanStatus        LoanStatus    `gorm:"column:loan_status;not null;default:embryo"`
	RepaymentDueDate  *time.Time    `gorm:"column:repayment_due_date;"`
	Tenure            LoanFrequency `gorm:"column:tenure;not null"`
	TenureValue       int           `gorm:"column:tenure_value"`
	CreatedAt         time.Time     `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt         *time.Time    `gorm:"column:updated_at;type:timestamptz;autoUpdateTime"`
}

func (LoanApplication) TableName() string {
	return "wallet_loan_applications"
}

type LoanApplicationStatusEvent struct {
	ID             string     `gorm:"column:id;type:text;primaryKey"`
	EventID        string     `gorm:"column:event_id;type:text;not null;uniqueIndex"`
	ApplicationRef string     `gorm:"column:application_ref;type:text;not null;index"`
	Status         LoanStatus `gorm:"column:status;type:text;not null"`
	CoreLoanID     *string    `gorm:"column:core_loan_id;type:text"`
	RawPayload     string     `gorm:"column:raw_payload;type:jsonb;not null"`
	ProcessedAt    time.Time  `gorm:"column:processed_at;type:timestamptz;not null"`
}

func (LoanApplicationStatusEvent) TableName() string {
	return "wallet_loan_application_status_events"
}

type CustomerEvent struct {
	ID             string                `gorm:"column:id;type:text;primaryKey"`
	EventID        string                `gorm:"column:event_id;type:text;not null;uniqueIndex"`
	CoreCustomerID string                `gorm:"column:core_customer_id;type:text;not null;index"`
	Status         models.CustomerStatus `gorm:"column:status;type:text;not null"`
	Username       string                `gorm:"username;type:text"`
	RawPayload     string                `gorm:"column:raw_payload;type:jsonb;not null"`
	ProcessedAt    time.Time             `gorm:"column:processed_at;type:timestamptz;not null"`
}

func (CustomerEvent) TableName() string {
	return "wallet_customer_events"
}

type Customer struct {
	Id                       int64     `gorm:"column:id"`
	Dob                      string    `gorm:"column:dob"`
	IdType                   string    `gorm:"column:id_type"`
	IdNumber                 string    `gorm:"column:id_number"`
	Nationalty               string    `gorm:"column:Nationalty"`
	StateOfOrigin            string    `gorm:"column:state_of_origin"`
	Religion                 string    `gorm:"column:Religion"`
	Occupation               string    `gorm:"column:Occupation"`
	Gender                   string    `gorm:"column:gender"`
	NextOfKinPhone           string    `gorm:"column:next_of_kin_phone"`
	NextOfKinAddress         string    `gorm:"column:next_of_kin_address"`
	NextOfKinRelationship    string    `gorm:"column:next_of_kin_Relationship"`
	CreatedAt                string    `gorm:"column:date"`
	UserId                   int64     `gorm:"column:user_id"`
	City                     string    `gorm:"column:city"`
	Education                string    `gorm:"column:education"`
	FullAddress              string    `gorm:"column:full_address"`
	HomeAddressState         string    `gorm:"column:home_address_state"`
	Landmark                 string    `gorm:"column:landmark"`
	LivingSince              string    `gorm:"column:living_since"`
	MaritalStatus            string    `gorm:"column:marital_status"`
	MobilePhone              string    `gorm:"column:mobile_phone"`
	Bvn                      string    `gorm:"column:bvn"`
	NextOfKinBvn             string    `gorm:"column:next_of_kin_bvn"`
	NextOfKinFirstName       string    `gorm:"column:next_of_kin_firstname"`
	NextOfKinLastName        string    `gorm:"column:next_of_kin_lastname"`
	NextOfKinMiddleName      string    `gorm:"column:next_of_kin_middlename"`
	NextOfKinPassport        string    `gorm:"column:next_of_kin_passport"`
	PlaceOfBirth             string    `gorm:"column:place_of_birth"`
	TypeOfHouse              string    `gorm:"column:type_of_house"`
	AccountName              string    `gorm:"column:account_name"`
	AccountNumber            string    `gorm:"column:account_number"`
	Bank                     string    `gorm:"column:bank"`
	NextOfKinLandmark        string    `gorm:"column:next_of_kin_landmark"`
	LastSentDate             time.Time `gorm:"column:last_sent_date"`
	LatitudeAddress          float64   `gorm:"column:latitude_address"`
	LongitudeAddress         float64   `gorm:"column:longitude_address"`
	AlternativeMobilePhone   string    `gorm:"column:alternative_mobile_phone"`
	BankCode                 string    `gorm:"column:bank_code"`
	DirectDebitMandateId     string    `gorm:"column:direct_debit_mandate_id"`
	DirectDebitMandateStatus string    `gorm:"column:direct_debit_mandate_status"`
	Deleted                  bool      `gorm:"column:deleted;default:false"`
}

type Loan struct {
	Id                          int64      `gorm:"column:id"`
	RefNo                       string     `gorm:"column:ref_no"`
	ExternalApplicationRef      *string    `gorm:"column:external_application_ref;type:varchar(100);uniqueIndex"`
	Amount                      float64    `gorm:"column:amount"`
	Frequency                   string     `gorm:"column:frequency"`
	LoanTerm                    string     `gorm:"column:loan_term"`
	LoanPurpose                 string     `gorm:"column:loan_purpose"`
	ReasonForRejection          string     `gorm:"column:reason_for_rejection"`
	PaymentDue                  bool       `gorm:"column:payment_due"`
	LastPayment                 *string    `gorm:"column:last_payment"`
	Status                      string     `gorm:"column:status"`
	Date                        string     `gorm:"column:date"`
	CustomerId                  int64      `gorm:"column:customer_id"`
	MadeById                    int64      `gorm:"column:made_by_id"`
	ValidationLevel             int64      `gorm:"column:validation_level"`
	ActualMoneyCollected        float64    `gorm:"column:actual_money_collected"`
	AmountToBePaid              float64    `gorm:"column:amount_to_be_paid"`
	ExpectedMoneyCollected      float64    `gorm:"column:expected_money_collected"`
	Installment                 float64    `gorm:"column:installment"`
	Profit                      float64    `gorm:"column:profit"`
	DisburseDate                *time.Time `gorm:"column:disburse_date"`
	Declined                    bool       `gorm:"column:declined"`
	DatePaid                    *string    `gorm:"column:date_paid"`
	DefaultAmount               float64    `gorm:"column:defaultamount"`
	ActualMoneyCollectedCapital float64    `gorm:"column:actual_money_collected_capital"`
	ActualMoneyCollectedProfit  float64    `gorm:"column:actual_money_collected_profit"`
	DefaultedDatePaid           *string    `gorm:"column:defaulted_date_paid"`
	LastChecked                 *string    `gorm:"column:last_checked"`
	CollectionDate              *string    `gorm:"column:collectionDate"`
	InitialCollectionDate       *string    `gorm:"column:initialcollectionDate"`
	LoanLiquidity               bool       `gorm:"column:loan_liquidity"`
	NonPerforming               bool       `gorm:"column:non_performing"`
	RatePercentage              string     `gorm:"column:ratePercentage"`
	TransferFromId              *int64     `gorm:"column:transfer_from_id"`
	CheckupApproved             bool       `gorm:"column:checkup_approved"`
	CheckupDate                 *string    `gorm:"column:checkup_date"`
	LastSentDate                *string    `gorm:"column:last_sent_date"`
	IsCreatorApproved           bool       `gorm:"column:is_creator_approved"`
	PaymentDueDate              string     `gorm:"column:payment_due_date;default:null"`
	ProductId                   int64      `gorm:"column:product_id"`
	ProcessingFeePaid           bool       `gorm:"column:processing_fee_paid"`

	InitialCollectionDateLagos *time.Time `gorm:"column:initialcollectionDate_lagos"`
	CollectionDateLagos        *time.Time `gorm:"column:collectionDate_lagos"`
	ProcessingFeeAmount        float64    `gorm:"column:processing_fee_amount"`
	ProcessingFeeDate          *time.Time `gorm:"column:processing_fee_date"`
	ProcessingFeePercentage    float64    `gorm:"column:processing_fee_percentage"`
}
