package baas

import "time"

type Bank struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type BankResponse struct {
	Status bool   `json:"status"`
	Banks  []Bank `json:"banks"`
}

type BankDetails struct {
	BankCode      string `json:"bankCode"`
	AccountName   string `json:"accountName"`
	AccountNumber string `json:"accountNumber"`
}

type BankDetailsResponse struct {
	Status  bool        `json:"status"`
	Account BankDetails `json:"account"`
}

type BulkTransferRecipientInfo struct {
	Amount        int64          `json:"amount" binding:"required,gt=0"`
	SortCode      string         `json:"sort_code" binding:"required"`
	Narration     *string        `json:"narration" binding:"omitempty,max=255"`
	AccountNumber string         `json:"account_number" binding:"required"`
	AccountName   *string        `json:"account_name" binding:"required,max=255"`
	Metadata      map[string]any `json:"metadata" binding:"omitempty"`
}

type BulkTransferRequest struct {
	RecipientInfo  []BulkTransferRecipientInfo `json:"recipient_info" binding:"required"`
	TransactionPin string                      `json:"transaction_pin" binding:"required"`
}

type BulkTransferResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		All      []BulkTransferResult `json:"all"`
		Rejected []BulkTransferResult `json:"rejected"`
		Accepted []BulkTransferResult `json:"accepted"`
	} `json:"data"`
}

type ProvidusBatchTransferResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		All      []ProvidusBatchTransferResult `json:"all"`
		Rejected []ProvidusBatchTransferResult `json:"rejected"`
		Accepted []ProvidusBatchTransferResult `json:"accepted"`
	} `json:"data"`
}

type ProvidusBatchTransferResult struct {
	Amount        int64  `json:"amount"`
	VAT           int64  `json:"vat"`
	SortCode      string `json:"sortCode"`
	Reference     string `json:"reference"`
	Narration     string `json:"narration"`
	AccountName   string `json:"accountName"`
	Fee           int64  `json:"fee"`
	AccountNumber string `json:"accountNumber"`
	Total         int64  `json:"total"`
}

type BulkTransferResult struct {
	Amount        int64  `json:"amount"`
	VAT           int64  `json:"vat"`
	SortCode      string `json:"sort_code"`
	Reference     string `json:"reference"`
	Narration     string `json:"narration"`
	AccountName   string `json:"accoun_name"`
	Fee           int64  `json:"fee"`
	AccountNumber string `json:"accoun_number"`
	Total         int64  `json:"total"`
}

type TransferRequest struct {
	Amount         float64        `json:"amount" binding:"required,gt=0"`
	SortCode       string         `json:"sort_code" binding:"required"`
	Narration      *string        `json:"narration" binding:"omitempty,max=255"`
	AccountNumber  string         `json:"account_number" binding:"required"`
	AccountName    *string        `json:"account_name" binding:"required,max=255"`
	Metadata       map[string]any `json:"metadata" binding:"omitempty"`
	TransactionPin string         `json:"transaction_pin" binding:"required"`
}

type TransferResponse struct {
	Status   bool           `json:"status"`
	Message  string         `json:"message"`
	Transfer TransferResult `json:"transfer"`
}

type TransferResult struct {
	Amount               float64                `json:"amount"`
	Charges              float64                `json:"charges"`
	Vat                  float64                `json:"vat"`
	Reference            string                 `json:"reference"`
	Total                float64                `json:"total"`
	Metadata             map[string]interface{} `json:"metadata"`
	SessionID            string                 `json:"sessionId"`
	Destination          string                 `json:"destination"`
	TransactionReference string                 `json:"transactionReference"`
	Description          string                 `json:"description"`
}

type optimusTokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type optimusTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken any    `json:"refreshToken"`
}

type optimusVerifyOTPRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	OTPToken    string `json:"otpToken"`
	Email       string `json:"email"`
	ReferenceID string `json:"referenceId"`
}

type OptimusPayload struct {
	RequestId         string `json:"RequestId"`
	Email             string `json:"Email"`
	Gender            string `json:"Gender"`
	MaritalStatus     string `json:"MaritalStatus"`
	MothersMaidenName string `json:"MothersMaidenName"`
	Address           string `json:"Address"`
	HouseNo           string `json:"HouseNo"`
	ProductId         string `json:"ProductId"`
	PhoneNumber       string `json:"PhoneNumber"`
	BVN               string `json:"Bvn"`
}

type ProvidusWalletActionPayload struct {
	Amount     int64       `json:"amount"`
	Reference  string      `json:"reference"`
	CustomerID string      `json:"customerId"`
	Metadata   interface{} `json:"metadata"`
}

type ProvidusWalletDebitResponse struct {
	Status  bool                            `json:"status"`
	Message string                          `json:"message"`
	Data    ProvidusWalletDebitResponseData `json:"data"`
}

type ProvidusWalletDebitResponseData struct {
	Amount           float64     `json:"amount"`
	Reference        string      `json:"reference"`
	CustomerID       string      `json:"customerId"`
	Metadata         interface{} `json:"metadata"`
	TransactionFee   int         `json:"transaction_fee"`
	CustomerWalletID string      `json:"customer_wallet_id"`
}

type ProvidusWalletCreditResponse struct {
	Status    bool    `json:"status"`
	Message   string  `json:"message"`
	Reference string  `json:"reference"`
	Amount    float64 `json:"amount"`
}

type ProvidusCustomerDetailsResponse struct {
	Status   bool             `json:"status"`
	Customer ProvidusCustomer `json:"customer"`
}

type ProvidusCustomer struct {
	ID               string            `json:"id"`
	BVN              string            `json:"bvn"`
	FirstName        string            `json:"firstName"`
	LastName         string            `json:"lastName"`
	BVNLastName      string            `json:"bvnLastName"`
	BVNFirstName     string            `json:"bvnFirstName"`
	NameMatch        bool              `json:"nameMatch"`
	Email            string            `json:"email"`
	DateOfBirth      string            `json:"dateOfBirth"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	WalletID         string            `json:"walletId"`
	Metadata         map[string]string `json:"metadata"`
	Tier             string            `json:"tier"`
	DeletedAt        *time.Time        `json:"deletedAt"`
	AccountNumber    string            `json:"accountNumber"`
	BookedBalance    float64           `json:"bookedBalance"`
	AvailableBalance float64           `json:"availableBalance"`
}

type ProvidusCustomerWalletResponse struct {
	Status bool           `json:"status"`
	Wallet ProvidusWallet `json:"wallet"`
}

type ProvidusWallet struct {
	Id                    string  `json:"id"`
	Type                  string  `json:"type"`
	Tier                  string  `json:"tier"`
	Status                string  `json:"status"`
	Email                 string  `json:"email"`
	CustomerId            string  `json:"customerId"`
	LastName              string  `json:"lastName"`
	FirstName             string  `json:"firstName"`
	BankName              string  `json:"bankName"`
	BankCode              string  `json:"bankCode"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
	AccountName           string  `json:"accountName"`
	PhoneNumber           string  `json:"phoneNumber"`
	PostNoCredit          bool    `json:"postNoCredit"`
	AccountNumber         string  `json:"accountNumber"`
	BookedBalance         float64 `json:"bookedBalance"`
	AvailableBalance      float64 `json:"availableBalance"`
	AccountReference      string  `json:"accountReference"`
	MinBalance            float64 `json:"minBalance"`
	MaxBalance            float64 `json:"maxBalance"`
	DailyTransactionLimit float64 `json:"dailyTransactionLimit"`
}

// OptimusAccountUpgradeRequest is the payload for Optimus's account-tier
// upgrade/KYC-completion endpoint. Fields that appeared null in the sample
// payload are pointers so an omitted value can be distinguished from an
// explicit empty string; date fields are kept as strings (matching
// OptimusPayload/OptimusBVNValidationRequest elsewhere in this file) since
// Optimus sends them as "yyyy-MM-ddTHH:mm:ss" with no timezone, which isn't
// directly compatible with time.Time's default JSON unmarshaling.
type OptimusAccountUpgradeRequest struct {
	AccountNumber        string                               `json:"AccountNumber"`
	RequestId            string                               `json:"RequestId"`
	Channel              int                                  `json:"Channel"`
	RequestType          int                                  `json:"RequestType"`
	PersonalDetails      OptimusAccountUpgradePersonalDetails `json:"PersonalDetails"`
	NokDetails           OptimusAccountUpgradeNokDetails      `json:"NokDetails"`
	AddressDetails       []OptimusAccountUpgradeAddressDetail `json:"AddressDetails"`
	Documents            []OptimusAccountUpgradeDocument      `json:"Documents"`
	IdentificationDetail OptimusAccountUpgradeIdentification  `json:"IdentificationDetail"`
	SocialMediaDetails   OptimusAccountUpgradeSocialMedia     `json:"SocialMediaDetails"`
	CitizenshipDetails   OptimusAccountUpgradeCitizenship     `json:"CitizenshipDetails"`
	EmploymentDetail     OptimusAccountUpgradeEmployment      `json:"EmploymentDetail"`
	MetaData             OptimusAccountUpgradeMetaData        `json:"MetaData"`
}

type OptimusAccountUpgradePersonalDetails struct {
	FirstName        *string `json:"FirstName"`
	MiddleName       *string `json:"MiddleName"`
	LastName         *string `json:"LastName"`
	MotherMaidenName string  `json:"MotherMaidenName"`
	DateOfBirth      *string `json:"DateOfBirth"`
	Title            string  `json:"Title"`
	MaritalStatus    int     `json:"MaritalStatus"`
	EmailAddress     *string `json:"EmailAddress"`
	PhoneNumber      *string `json:"PhoneNumber"`
}

type OptimusAccountUpgradeNokDetails struct {
	FullName     string `json:"FullName"`
	Relationship string `json:"Relationship"`
	PhoneNumber  string `json:"PhoneNumber"`
	Email        string `json:"Email"`
	Gender       string `json:"Gender"`
	Dob          string `json:"Dob"`
}

// OptimusAccountUpgradeAddressDetail is repeated per AddressType (e.g. 0 =
// residential, 1 = office, 2 = other) - fields are pointers since the sample
// payload shows some address entries fully populated and others (e.g. the
// "other" address) with most fields null.
type OptimusAccountUpgradeAddressDetail struct {
	AddressType     int     `json:"AddressType"`
	HouseNumber     *string `json:"HouseNumber"`
	AddressLine1    string  `json:"AddressLine1"`
	AddressLine2    *string `json:"AddressLine2"`
	LocalGovernment *string `json:"LocalGovernment"`
	City            *string `json:"City"`
	State           *string `json:"State"`
	Country         string  `json:"Country"`
	ZipCode         *string `json:"ZipCode"`
}

type OptimusAccountUpgradeDocument struct {
	DocumentName  string `json:"DocumentName"`
	DocumentType  int    `json:"DocumentType"`
	FileExtension string `json:"FileExtension"`
	Base64Doc     string `json:"base64Doc"`
}

type OptimusAccountUpgradeIdentification struct {
	Nin        string  `json:"Nin"`
	IdNo       string  `json:"IdNo"`
	IdType     int     `json:"IdType"`
	IssueDate  *string `json:"IssueDate"`
	ExpiryDate *string `json:"ExpiryDate"`
}

type OptimusAccountUpgradeSocialMedia struct {
	LinkedIn  string  `json:"LinkedIn"`
	Facebook  string  `json:"Facebook"`
	Instagram string  `json:"Instagram"`
	Tiktok    string  `json:"Tiktok"`
	Twitter   string  `json:"Twitter"`
	Thread    *string `json:"Thread"`
}

type OptimusAccountUpgradeCitizenship struct {
	ForeignTaxId        string `json:"ForeignTaxId"`
	CountryTaxResidence string `json:"CountryTaxResidence"`
	NoReasonForTinClass string `json:"NoReasonForTinClass"`
	NoReasonCause       string `json:"NoReasonCause"`
	FatcaAccepted       bool   `json:"FatcaAccepted"`
	PhoneNumber         string `json:"PhoneNumber"`
}

type OptimusAccountUpgradeEmployment struct {
	EmploymentStatus  int    `json:"EmploymentStatus"`
	YearsOfEmployment string `json:"YearsOfEmployment"`
	AnnualIncome      string `json:"AnnualIncome"`
	EmployersName     string `json:"EmployersName"`
	NatureOfBusiness  int    `json:"NatureOfBusiness"`
	Occupation        string `json:"Occupation"`
	SourceOfWealth    string `json:"SourceOfWealth"`
	EmployersAddress  string `json:"EmployersAddress"`
	EmploymentDate    string `json:"EmploymentDate"`
}

type OptimusAccountUpgradeMetaData struct {
	NotificationPreference *string `json:"NotificationPreference"`
	PurposeOfAccount       string  `json:"PurposeOfAccount"`
	OtherReasons           *string `json:"OtherReasons"`
}
