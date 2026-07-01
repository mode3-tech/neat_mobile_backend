package baas

import "time"

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
