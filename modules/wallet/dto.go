package wallet

type BankDetailsQuery struct {
	AccountNumber string `form:"account_number" binding:"required"`
	BankCode      string `form:"bank_code" binding:"required"`
}

type BankDetailsResponse struct {
	Status  bool        `json:"status"`
	Account BankDetails `json:"account"`
}

type TransferRequest struct {
	Amount         int64          `json:"amount" binding:"required,gt=0"`
	SortCode       string         `json:"sortCode" binding:"required"`
	Narration      *string        `json:"narration" binding:"omitempty,max=255"`
	AccountNumber  string         `json:"accountNumber" binding:"required"`
	AccountName    *string        `json:"accountName" binding:"omitempty,max=255"`
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

type AddBeneficiaryRequest struct {
	BankCode      string `json:"bank_code" binding:"required"`
	AccountNumber string `json:"account_number" binding:"required"`
	AccountName   string `json:"account_name" binding:"required"`
}

type AddBeneficiaryResponse struct {
	Status      bool        `json:"status"`
	Message     string      `json:"message"`
	Beneficiary Beneficiary `json:"beneficiary"`
}

// type FetchBeneficiariesQuery struct {
// 	WalletID string `form:"wallet_id" binding:"required"`
// }

type FetchBeneficiariesResponse struct {
	Status        bool                        `json:"status"`
	Message       string                      `json:"message"`
	Beneficiaries []BeneficiaryResponseStruct `json:"beneficiaries"`
}

type BeneficiaryResponseStruct struct {
	WalletID      string `json:"wallet_id"`
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

type ProvidusCredit struct {
	AccountNumber          string `json:"accountNumber"`
	TransactionAmount      string `json:"transactionAmount"`
	SettledAmount          string `json:"settledAmount"`
	Currency               string `json:"currency"`
	TranType               string `json:"tranType"`
	TranRemarks            string `json:"tranRemarks"`
	TranDate               string `json:"tranDate"`
	SessionID              string `json:"sessionId"`
	TranID                 string `json:"tranId"`
	InitiationTranRef      string `json:"initiationTranRef"`
	OriginatorAccountName  string `json:"originatorAccountName"`
	OriginatorAccountNo    string `json:"originatorAccountNumber"`
	BeneficiaryAccountName string `json:"beneficiaryAccountName"`
	BeneficiaryAccountNo   string `json:"beneficiaryAccountNumber"`
}
