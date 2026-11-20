package baas

import (
	"bytes"
	"encoding/json"
	"time"
)

type AlatResponseCode string

const (
	AlatResponseCodeInvalidBVN                  AlatResponseCode = "InvalidBvn"
	AlatResponseCodeFacialValidationSuccessfull AlatResponseCode = "FacialVlidationSuccessfull"
	AlatResponseCodeFacialValidationFailed      AlatResponseCode = "FacialVlidationFailed"
	AlatResponseCodeError                       AlatResponseCode = "Error"
	AlatResponseCodeForeignNumber               AlatResponseCode = "ForeignNumber"
	AlatResponseCodeLivenessCheckFailed         AlatResponseCode = "LivenessCheckFailed"
)

type AlatHTTPStatusCode string

const (
	AlatHTTPStatusCodeContinue                      AlatHTTPStatusCode = "Continue"
	AlatHTTPStatusCodeSwitchingProtocols            AlatHTTPStatusCode = "SwitchingProtocols"
	AlatHTTPStatusCodeProcessing                    AlatHTTPStatusCode = "Processing"
	AlatHTTPStatusCodeEarlyHints                    AlatHTTPStatusCode = "EarlyHints"
	AlatHTTPStatusCodeOK                            AlatHTTPStatusCode = "OK"
	AlatHTTPStatusCodeCreated                       AlatHTTPStatusCode = "Created"
	AlatHTTPStatusCodeAccepted                      AlatHTTPStatusCode = "Accepted"
	AlatHTTPStatusCodeNonAuthoritativeInformation   AlatHTTPStatusCode = "NonAuthoritativeInformation"
	AlatHTTPStatusCodeNoContent                     AlatHTTPStatusCode = "NoContent"
	AlatHTTPStatusCodeResetContent                  AlatHTTPStatusCode = "ResetContent"
	AlatHTTPStatusCodePartialContent                AlatHTTPStatusCode = "PartialContent"
	AlatHTTPStatusCodeMultiStatus                   AlatHTTPStatusCode = "MultiStatus"
	AlatHTTPStatusCodeAlreadyReported               AlatHTTPStatusCode = "AlreadyReported"
	AlatHTTPStatusCodeIMUsed                        AlatHTTPStatusCode = "IMUsed"
	AlatHTTPStatusCodeMultipleChoices               AlatHTTPStatusCode = "MultipleChoices"
	AlatHTTPStatusCodeMovedPermanently              AlatHTTPStatusCode = "MovedPermanently"
	AlatHTTPStatusCodeFound                         AlatHTTPStatusCode = "Found"
	AlatHTTPStatusCodeSeeOther                      AlatHTTPStatusCode = "SeeOther"
	AlatHTTPStatusCodeNotModified                   AlatHTTPStatusCode = "NotModified"
	AlatHTTPStatusCodeUseProxy                      AlatHTTPStatusCode = "UseProxy"
	AlatHTTPStatusCodeUnused                        AlatHTTPStatusCode = "Unused"
	AlatHTTPStatusCodeTemporaryRedirect             AlatHTTPStatusCode = "TemporaryRedirect"
	AlatHTTPStatusCodePermanentRedirect             AlatHTTPStatusCode = "PermanentRedirect"
	AlatHTTPStatusCodeBadRequest                    AlatHTTPStatusCode = "BadRequest"
	AlatHTTPStatusCodeUnauthorized                  AlatHTTPStatusCode = "Unauthorized"
	AlatHTTPStatusCodePaymentRequired               AlatHTTPStatusCode = "PaymentRequired"
	AlatHTTPStatusCodeForbidden                     AlatHTTPStatusCode = "Forbidden"
	AlatHTTPStatusCodeNotFound                      AlatHTTPStatusCode = "NotFound"
	AlatHTTPStatusCodeMethodNotAllowed              AlatHTTPStatusCode = "MethodNotAllowed"
	AlatHTTPStatusCodeNotAcceptable                 AlatHTTPStatusCode = "NotAcceptable"
	AlatHTTPStatusCodeProxyAuthenticationRequired   AlatHTTPStatusCode = "ProxyAuthenticationRequired"
	AlatHTTPStatusCodeRequestTimeout                AlatHTTPStatusCode = "RequestTimeout"
	AlatHTTPStatusCodeConflict                      AlatHTTPStatusCode = "Conflict"
	AlatHTTPStatusCodeGone                          AlatHTTPStatusCode = "Gone"
	AlatHTTPStatusCodeLengthRequired                AlatHTTPStatusCode = "LengthRequired"
	AlatHTTPStatusCodePreconditionFailed            AlatHTTPStatusCode = "PreconditionFailed"
	AlatHTTPStatusCodeRequestEntityTooLarge         AlatHTTPStatusCode = "RequestEntityTooLarge"
	AlatHTTPStatusCodeRequestUriTooLong             AlatHTTPStatusCode = "RequestUriTooLong"
	AlatHTTPStatusCodeUnsupportedMediaType          AlatHTTPStatusCode = "UnsupportedMediaType"
	AlatHTTPStatusCodeRequestedRangeNotSatisfiable  AlatHTTPStatusCode = "RequestedRangeNotSatisfiable"
	AlatHTTPStatusCodeExpectationFailed             AlatHTTPStatusCode = "ExpectationFailed"
	AlatHTTPStatusCodeMisdirectedRequest            AlatHTTPStatusCode = "MisdirectedRequest"
	AlatHTTPStatusCodeUnprocessableEntity           AlatHTTPStatusCode = "UnprocessableEntity"
	AlatHTTPStatusCodeLocked                        AlatHTTPStatusCode = "Locked"
	AlatHTTPStatusCodeFailedDependency              AlatHTTPStatusCode = "FailedDependency"
	AlatHTTPStatusCodeUpgradeRequired               AlatHTTPStatusCode = "UpgradeRequired"
	AlatHTTPStatusCodePreconditionRequired          AlatHTTPStatusCode = "PreconditionRequired"
	AlatHTTPStatusCodeTooManyRequests               AlatHTTPStatusCode = "TooManyRequests"
	AlatHTTPStatusCodeRequestHeaderFieldsTooLarge   AlatHTTPStatusCode = "RequestHeaderFieldsTooLarge"
	AlatHTTPStatusCodeUnavailableForLegalReasons    AlatHTTPStatusCode = "UnavailableForLegalReasons"
	AlatHTTPStatusCodeInternalServerError           AlatHTTPStatusCode = "InternalServerError"
	AlatHTTPStatusCodeNotImplemented                AlatHTTPStatusCode = "NotImplemented"
	AlatHTTPStatusCodeBadGateway                    AlatHTTPStatusCode = "BadGateway"
	AlatHTTPStatusCodeServiceUnavailable            AlatHTTPStatusCode = "ServiceUnavailable"
	AlatHTTPStatusCodeGatewayTimeout                AlatHTTPStatusCode = "GatewayTimeout"
	AlatHTTPStatusCodeHttpVersionNotSupported       AlatHTTPStatusCode = "HttpVersionNotSupported"
	AlatHTTPStatusCodeVariantAlsoNegotiates         AlatHTTPStatusCode = "VariantAlsoNegotiates"
	AlatHTTPStatusCodeInsufficientStorage           AlatHTTPStatusCode = "InsufficientStorage"
	AlatHTTPStatusCodeLoopDetected                  AlatHTTPStatusCode = "LoopDetected"
	AlatHTTPStatusCodeNotExtended                   AlatHTTPStatusCode = "NotExtended"
	AlatHTTPStatusCodeNetworkAuthenticationRequired AlatHTTPStatusCode = "NetworkAuthenticationRequired"
)

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
	PhoneNumber string `json:"phoneNo"`
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

type AlatAccountCreationResponseData struct {
	AccountGenerationStatus   string `json:"accountGenerationStatus"`
	TrackingID                string `json:"trackingId"`
	AddressVerificationStatus string `json:"addressVerificationStatus"`
}

// Account creation is called PartnerOnboardingRequestModel on their API documentation
type AlatAccountCreationResponse struct {
	Message    string                           `json:"message"`
	Status     bool                             `json:"status"`
	Code       AlatResponseCode                 `json:"code"`
	StatusCode AlatHTTPStatusCode               `json:"statusCode"`
	Errors     []string                         `json:"errors"`
	Data       *AlatAccountCreationResponseData `json:"data"`
}

// Account creation is called PartnerOnboardingRequestModel on their API documentation
// This payload creates a Tier2 account
type AlatAccountCreationRequest struct {
	BVN                string              `json:"bvn"`
	NIN                string              `json:"nin"`
	PhoneNumber        string              `json:"phoneNumber"`
	EmailAddress       string              `json:"emailAddress"`
	ResidentialAddress *ResidentialAddress `json:"residentialAddress,omitempty"` // omitted for tier 2; added at the tier 3 upgrade
	LiveImageOfFace    string              `json:"liveImageOfFace"`
	CorrelationID      string              `json:"correlationId"`
}

type ResidentialAddress struct {
	BuildingNumber        string `json:"buildingNumber"`
	Apartment             string `json:"apartment"`
	Street                string `json:"street"`
	City                  string `json:"city"`
	Town                  string `json:"town"`
	State                 string `json:"state"`
	LGA                   string `json:"lga"`
	LCDA                  string `json:"lcda"`
	Landmark              string `json:"landmark"`
	AdditionalInformation string `json:"additionalInformation"`
	Country               string `json:"country"`
	FullAddress           string `json:"fullAddress"`
	PostalCode            string `json:"postalCode"`
}

type CountryList struct {
	ID          int64  `json:"id"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
}

type StateList struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FinacleCode string `json:"finacleCode"`
	Country     string `json:"country"`
}

type LGAList struct {
	LGAID   int64  `json:"lgaId"`
	StateID int64  `json:"stateId"`
	Name    string `json:"name"`
}

type LCDAList struct {
	LCDAID int64  `json:"lcdaId"`
	LGAID  int64  `json:"lgaId"`
	Name   string `json:"name"`
}

type CityList struct {
	ID      int64  `json:"id"`
	StateID int64  `json:"stateId"`
	Name    string `json:"name"`
}

type HousingType struct {
	ID    int64  `json:"id"`
	Value string `json:"value"`
	Code  string `json:"code"`
}

// Called ContactDetailDropDownModel on their documentation
type AlatAddressDropdown struct {
	CountryModel AddressList `json:"countryModel"`
}

// Response envelope of GetDropDownList
type AlatAddressDropdownResponse struct {
	Message    string               `json:"message"`
	Status     bool                 `json:"status"`
	Code       AlatResponseCode     `json:"code"`
	StatusCode AlatHTTPStatusCode   `json:"statusCode"`
	Errors     []string             `json:"errors"`
	Data       *AlatAddressDropdown `json:"data"`
}

// Called CountryModel on their documentation
type AddressList struct {
	CountryList  []CountryList `json:"countryList"`
	StateList    []StateList   `json:"stateList"`
	LGAList      []LGAList     `json:"lgaList"`
	LCDAList     []LCDAList    `json:"lcdaList"`
	CityList     []CityList    `json:"cityList"`
	HousingTypes []HousingType `json:"housingTypes"`
}

type AlatAccountDetailsResponseData struct {
	AccountNumber string `json:"accountNumber"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	Email         string `json:"email"`
	PhoneNumber   string `json:"phoneNumber"`
}

// Called GetPartnershipAccountDetailsResponseResponseModel on their documentation
type AlatAccountDetailsResponse struct {
	Message    string                          `json:"message"`
	Status     bool                            `json:"status"`
	Code       AlatResponseCode                `json:"code"`
	StatusCode AlatHTTPStatusCode              `json:"statusCode"`
	Errors     []string                        `json:"errors"`
	Data       *AlatAccountDetailsResponseData `json:"data"`
}

// Called B2BGetAccountV2Reponse on their documentation
type alatAccount struct {
	WalletNumber     string `json:"walletNumber"`
	AvailableBalance string `json:"availableBalance"`
	WalletStatus     string `json:"walletStatus"`
	AccountType      string `json:"accountType"`
}

type AlatWalletDetailsResponse struct {
	Result     *alatAccount `json:"result"`
	Successful bool         `json:"successful"`
	Message    string       `json:"message"`
}

// AlatTransactionType is ALAT's TransactionType enum.
type AlatTransactionType string

const (
	AlatTransactionTypeInterBank   AlatTransactionType = "InterBank"
	AlatTransactionTypeIntraBank   AlatTransactionType = "IntraBank"
	AlatTransactionTypeIntraBankFx AlatTransactionType = "IntraBankFx"
	AlatTransactionTypeAirtime     AlatTransactionType = "Airtime"
	AlatTransactionTypeData        AlatTransactionType = "Data"
	AlatTransactionTypeBills       AlatTransactionType = "Bills"
	AlatTransactionTypeDollarCard  AlatTransactionType = "DollarCard"
	AlatTransactionTypeMovies      AlatTransactionType = "Movies"
	AlatTransactionTypeEvents      AlatTransactionType = "Events"
	AlatTransactionTypeRemita      AlatTransactionType = "Remita"
	AlatTransactionTypeDefault     AlatTransactionType = "Default"
)

// AlatTransactionStatus is ALAT's TransactionStatus enum ("Successfull" is ALAT's spelling).
type AlatTransactionStatus string

const (
	AlatTransactionStatusDefault    AlatTransactionStatus = "Default"
	AlatTransactionStatusSuccessful AlatTransactionStatus = "Successfull"
	AlatTransactionStatusFailed     AlatTransactionStatus = "Failed"
	AlatTransactionStatusPending    AlatTransactionStatus = "Pending"
)

// AlatCreditType is ALAT's CreditType enum.
type AlatCreditType string

const (
	AlatCreditTypeDefault AlatCreditType = "Default"
	AlatCreditTypeCredit  AlatCreditType = "Credit"
	AlatCreditTypeDebit   AlatCreditType = "Debit"
)

// AlatTransactionHistoryRequest is ALAT's TransactionhistoryV2Request.
type AlatTransactionHistoryRequest struct {
	AccountNumber string     `json:"accountNumber"`
	From          *time.Time `json:"from,omitempty"`
	To            *time.Time `json:"to,omitempty"`
	KeyWord       string     `json:"keyWord,omitempty"`
}

// AlatTransactionHistory is ALAT's TransactionHistoryModel.
// Date fields stay strings: ALAT's date-times often lack a timezone, which
// time.Time can't decode.
type AlatTransactionHistory struct {
	Title                    string                `json:"title"`
	Amount                   float64               `json:"amount"`
	Type                     AlatTransactionType   `json:"type"`
	Date                     string                `json:"date"`
	TransactionDate          string                `json:"transactionDate"`
	Narration                string                `json:"narration"`
	Status                   AlatTransactionStatus `json:"status"`
	CreditType               AlatCreditType        `json:"creditType"`
	Sender                   string                `json:"sender"`
	SenderAccountNumber      string                `json:"senderAccountNumber"`
	DestinationBank          string                `json:"destinationBank"`
	DestinationAccountNumber string                `json:"destinationAccountNumber"`
	ReceiverName             string                `json:"recieverName"` // ALAT's spelling
	ReferenceID              string                `json:"referenceId"`
	IsViewReceiptEnabled     bool                  `json:"isViewReceiptEnabled"`
	TranID                   string                `json:"tranId"`
}

// AlatTransactionHistoryResponse is ALAT's TransactionHistoryModelListServiceResponse.
type AlatTransactionHistoryResponse struct {
	Result     []AlatTransactionHistory `json:"result"`
	Successful bool                     `json:"successful"`
	Message    string                   `json:"message"`
}

// Called Banks on their documentation
type AlatBank struct {
	BankName string `json:"bankName"`
	BankCode string `json:"bankCode"`
}

// Called BanksEnvelope on their documentation
type AlatBanksResponse struct {
	Result        AlatBankList `json:"result"`
	ErrorMessage  string       `json:"errorMessage"`
	ErrorMessages []string     `json:"errorMessages"`
	HasError      bool         `json:"hasError"`
	TimeGenerated string       `json:"timeGenerated"`
}

// AlatBankList is the banks result. Their documentation shows a single bank
// object where a list is expected, so both shapes are accepted.
type AlatBankList []AlatBank

func (l *AlatBankList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var one AlatBank
		if err := json.Unmarshal(trimmed, &one); err != nil {
			return err
		}
		*l = AlatBankList{one}
		return nil
	}
	var many []AlatBank
	if err := json.Unmarshal(trimmed, &many); err != nil {
		return err
	}
	*l = many
	return nil
}

// Called ChargeFee on their documentation
type AlatChargeFee struct {
	ID              int     `json:"id"`
	ChargeFeeName   string  `json:"chargeFeeName"`
	TransactionType int     `json:"transactionType"`
	Charge          float64 `json:"charge"`
	Lower           float64 `json:"lower"`
	Upper           float64 `json:"upper"`
}

// Called AccountNameEnquiry on their documentation
type AlatAccountNameEnquiry struct {
	BankCode              string          `json:"bankCode"`
	AccountName           string          `json:"accountName"`
	AccountNumber         string          `json:"accountNumber"`
	Currency              string          `json:"currency"`
	TermsAndConditions    string          `json:"termsAndConditions"`
	TermsAndConditionsURL string          `json:"termsAndConditionsUrl"`
	ChargeFee             []AlatChargeFee `json:"chargeFee"`
}

// Called AccountNameEnquiryEnvelope on their documentation
type AlatAccountNameEnquiryResponse struct {
	Result        *AlatAccountNameEnquiry `json:"result"`
	ErrorMessage  string                  `json:"errorMessage"`
	ErrorMessages []string                `json:"errorMessages"`
	HasError      bool                    `json:"hasError"`
	TimeGenerated string                  `json:"timeGenerated"`
}

// Called NameEnquiryResponse on their documentation
type AlatWalletNameEnquiry struct {
	BankCode      string `json:"bankCode"`
	AccountName   string `json:"accountName"`
	AccountNumber string `json:"accountNumber"`
	Currency      string `json:"currency"`
}

// Called NameEnquiryResponseEnvelope on their documentation
type AlatWalletNameEnquiryResponse struct {
	Result        *AlatWalletNameEnquiry `json:"result"`
	ErrorMessage  string                 `json:"errorMessage"`
	ErrorMessages []string               `json:"errorMessages"`
	HasError      bool                   `json:"hasError"`
	TimeGenerated string                 `json:"timeGenerated"`
}

// Called NIPCharges on their documentation
type AlatNIPCharges struct {
	ChargeFees            []AlatChargeFee `json:"chargeFees"`
	TermsAndConditions    string          `json:"termsAndConditions"`
	TermsAndConditionsURL string          `json:"termsAndConditionsUrl"`
}

// Called NIPChargesEnvelope on their documentation
type AlatNIPChargesResponse struct {
	Result        *AlatNIPCharges `json:"result"`
	ErrorMessage  string          `json:"errorMessage"`
	ErrorMessages []string        `json:"errorMessages"`
	HasError      bool            `json:"hasError"`
	TimeGenerated string          `json:"timeGenerated"`
}

// Called ClientTransferRequestDto on their documentation.
// SecurityInfo is our signed value; ALAT echoes it to the Authentication Callback.
type AlatClientTransferRequest struct {
	SecurityInfo             string  `json:"securityInfo"`
	Amount                   float64 `json:"amount"`
	DestinationBankCode      string  `json:"destinationBankCode"`
	DestinationBankName      string  `json:"destinationBankName"`
	DestinationAccountNumber string  `json:"destinationAccountNumber"`
	DestinationAccountName   string  `json:"destinationAccountName"`
	SourceAccountNumber      string  `json:"sourceAccountNumber"`
	Narration                string  `json:"narration"`
	TransactionReference     string  `json:"transactionReference"`
	UseCustomNarration       bool    `json:"useCustomNarration"`
}

// Called GlobalResponse on their documentation
type AlatTransferResult struct {
	Status                       string `json:"status"`
	Message                      string `json:"message"`
	Narration                    string `json:"narration"`
	TransactionReference         string `json:"transactionReference"`
	PlatformTransactionReference string `json:"platformTransactionReference"`
	TransactionStan              string `json:"transactionStan"`
	OriginalTxnTransactionDate   string `json:"orinalTxnTransactionDate"` // ALAT's spelling
}

// Called GlobalResponseEnvelope on their documentation
type AlatClientTransferResponse struct {
	Result        *AlatTransferResult `json:"result"`
	ErrorMessage  string              `json:"errorMessage"`
	ErrorMessages []string            `json:"errorMessages"`
	HasError      bool                `json:"hasError"`
	TimeGenerated string              `json:"timeGenerated"`
}

type AlatTransferStatus struct {
	Title   string              `json:"title"`
	Message string              `json:"message"`
	Data    *AlatTransferResult `json:"data"`
	Request int                 `json:"request"`
}

// Called ClientTransferCallBackResponseEnvelope on their documentation
type AlatTransferStatusResponse struct {
	Result        *AlatTransferStatus `json:"result"`
	ErrorMessage  string              `json:"errorMessage"`
	ErrorMessages []string            `json:"errorMessages"`
	HasError      bool                `json:"hasError"`
	TimeGenerated string              `json:"timeGenerated"`
}

// Called FundWalletRequest on their documentation.
// SecurityInfo is our signed value; ALAT echoes it to the Authentication Callback.
type AlatFundWalletRequest struct {
	SecurityInfo             string  `json:"securityInfo"`
	DestinationAccountNumber string  `json:"destinationAccountNumber"`
	Amount                   float64 `json:"amount"`
	Narration                string  `json:"narration"`
	TransactionReference     string  `json:"transactionReference"`
	UseCustomNarration       bool    `json:"useCustomNarration"`
}

// Called PartnerAddressModel on their documentation
type AlatTier3UpgradeRequest struct {
	ResidentialAddress ResidentialAddress `json:"residentialAddress"`
	AccountNumber      string             `json:"accountNumber"`
}

// Called PartnerAddressModelV2 on their documentation
type AlatTier2UpgradeRequest struct {
	AccountNumber   string `json:"accountNumber"`
	NIN             string `json:"nin"`
	BVN             string `json:"bvn"`
	LiveImageOfFace string `json:"liveImageOfFace"`
}

// Called ResponseModel on their documentation
type AlatAccountUpgradeResponse struct {
	Message    string             `json:"message"`
	Status     bool               `json:"status"`
	Code       AlatResponseCode   `json:"code"`
	StatusCode AlatHTTPStatusCode `json:"statusCode"`
	Errors     []string           `json:"errors"`
}

// Called PartnerKycStatusResponse on their documentation
type AlatKYCStatus struct {
	AccountNumber             string `json:"accountNumber"`
	AccountName               string `json:"accountName"`
	AccountTier               string `json:"accountTier"`
	AccountStatus             string `json:"accountStatus"`
	RestrictionStatus         string `json:"restrictionStatus"`
	AddressVerificationStatus string `json:"addressVerificationStatus"`
}

// Called PartnerKycStatusResponseResponseModel on their documentation
type AlatKYCStatusResponse struct {
	Message    string             `json:"message"`
	Status     bool               `json:"status"`
	Code       AlatResponseCode   `json:"code"`
	StatusCode AlatHTTPStatusCode `json:"statusCode"`
	Errors     []string           `json:"errors"`
	Data       *AlatKYCStatus     `json:"data"`
}

// Static Wallet Service: response of GET /api/v1/bulk-accounts/summary
type AlatBulkAccountsSummaryResponse struct {
	TotalAccountsCreated              int64 `json:"totalAccountsCreated"`
	TotalInactiveAccounts             int64 `json:"totalInactiveAccounts"`
	TotalActiveAccounts               int64 `json:"totalActiveAccounts"`
	TotalAccountsAwaitingDisbursement int64 `json:"totalAccountsAwaitingDisbursement"`
}

// Called BalanceEnquiry on their documentation
type AlatBalanceEnquiryRequest struct {
	AccountNumber string `json:"accountNumber"`
}

// Static Wallet Service: response of POST /api/v1/bulk-accounts/balance
type AlatBalanceEnquiryResponse struct {
	AccountNumber string  `json:"accountNumber"`
	Balance       float64 `json:"balance"`
	Date          string  `json:"date"`
}

// Called InitiateGetCustomerStatementRequest on their documentation
type AlatInitiateStatementRequest struct {
	AccountNumber string    `json:"accountNumber"`
	DateFrom      time.Time `json:"dateFrom"`
	DateTo        time.Time `json:"dateTo"`
}

// Called InitiateGetCustomerStatementResponse.data on their documentation
type AlatStatementReference struct {
	ReferenceID string `json:"referenceId"`
}

// Called InitiateGetCustomerStatementResponse on their documentation
type AlatInitiateStatementResponse struct {
	Data    *AlatStatementReference `json:"data"`
	Message string                  `json:"message"`
	Status  bool                    `json:"status"`
}

// Called GetCustomerTransactionsRequest on their documentation
type AlatCustomerTransactionsRequest struct {
	ReferenceID string `json:"referenceId"`
}

// Called GetCustomerTransactionsResponse on their documentation.
// Its items are the same TransactionHistoryModel as transaction history
// (minus transactionDate, which stays empty).
type AlatCustomerTransactionsResponse struct {
	Data    []AlatTransactionHistory `json:"data"`
	Message string                   `json:"message"`
	Status  bool                     `json:"status"`
}

// Called CustomerConsentCallBackRequest on their documentation.
// Listed in the spec without an endpoint; presumably sent to us when the
// customer approves or rejects statement access.
type AlatCustomerConsentCallbackRequest struct {
	Reference   string `json:"reference"`
	CustomerCIF string `json:"customerCif"`
	IsApproved  bool   `json:"isApproved"`
}

// Called CustomerConsentCallBackResponse on their documentation
type AlatCustomerConsentCallbackResponse struct {
	ResponseMessage string `json:"responseMessage"`
	ResponseCode    string `json:"responseCode"`
}

// Called ResponseModel on their documentation: the envelope without data.
type AlatResponseModel struct {
	Message    string             `json:"message"`
	Status     bool               `json:"status"`
	Code       AlatResponseCode   `json:"code"`
	StatusCode AlatHTTPStatusCode `json:"statusCode"`
	Errors     []string           `json:"errors"`
}

// Called TierOneBVNRequest on their documentation
type AlatTier1BVNRequest struct {
	PhoneNumber   string `json:"phoneNumber"`
	Email         string `json:"email"`
	BVN           string `json:"bvn"`
	CorrelationID string `json:"correlationId"`
}

// Called TierOneNinRequest on their documentation
type AlatTier1NINRequest struct {
	PhoneNumber   string `json:"phoneNumber"`
	Email         string `json:"email"`
	NIN           string `json:"nin"`
	CorrelationID string `json:"correlationId"`
}

// Called PatnershipRequestWithBvn on their documentation (Wallet Creation API - BVN, Step 1).
// The documented response is a ResponseModel with no data; we still read
// data.trackingId, which Step 2 needs.
type AlatPartnershipRequestWithBvn struct {
	PhoneNumber string `json:"phoneNumber"`
	Email       string `json:"email"`
	BVN         string `json:"bvn"`
}

// Called PatnershipRequestV2 on their documentation (Wallet Creation API - BVN, Step 2)
type AlatValidateBvnRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	OTP         string `json:"otp"`
	TrackingID  string `json:"trackingId"`
}

// Called ResendOtpModel on their documentation (Wallet Creation API - BVN)
type AlatResendOtpRequest struct {
	PhoneNumber string `json:"phoneNumber,omitempty"`
	TrackingID  string `json:"trackingId,omitempty"`
}

// Called PndType on their documentation
type AlatPndType string

const (
	AlatPndTypeLift  AlatPndType = "LiftPnd"
	AlatPndTypePlace AlatPndType = "PlacePnd"
)

// Called PndRequest on their documentation (Wallet Creation API - BVN, Step 4):
// lift or place the post-no-debit (PND) restriction ALAT puts on new accounts
type AlatPndRequest struct {
	PndType       AlatPndType `json:"pndType"`
	AccountNumber string      `json:"accountNumber"`
}

// ALAT documents code/statusCode as enum names ("InvalidBvn", "Continue"),
// but live responses also send them as numbers ("code": 0). Both decode to a
// string so a numeric code doesn't fail the whole response.

func (c *AlatResponseCode) UnmarshalJSON(data []byte) error {
	s, err := stringOrNumber(data)
	*c = AlatResponseCode(s)
	return err
}

func (c *AlatHTTPStatusCode) UnmarshalJSON(data []byte) error {
	s, err := stringOrNumber(data)
	*c = AlatHTTPStatusCode(s)
	return err
}

func stringOrNumber(data []byte) (string, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	if trimmed[0] == '"' {
		var s string
		err := json.Unmarshal(trimmed, &s)
		return s, err
	}
	var n json.Number
	if err := json.Unmarshal(trimmed, &n); err != nil {
		return "", err
	}
	return n.String(), nil
}
