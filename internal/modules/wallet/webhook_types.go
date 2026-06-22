package wallet

type BaaSEventEnvelope struct {
	Event string `json:"event"`
}

type CustomerBankTransferWebhook struct {
	Event string                   `json:"event"`
	Data  CustomerBankTransferData `json:"data"`
}

type CustomerBankTransferData struct {
	Amount               float64                `json:"amount"`
	Charges              float64                `json:"charges"`
	VAT                  float64                `json:"vat"`
	Reference            string                 `json:"reference"`
	CustomerID           string                 `json:"customerId"`
	Status               string                 `json:"status"`
	Total                float64                `json:"total"`
	TransactionReference string                 `json:"transactionReference"`
	Metadata             map[string]interface{} `json:"metadata"`
	Description          string                 `json:"description"`
	Destination          string                 `json:"destination"`
}

type AccountFundedWebhook struct {
	Event string            `json:"event"`
	Data  AccountFundedData `json:"data"`
}

type AccountFundedData struct {
	UserID               string `json:"userId"`
	Amount               string `json:"amount"`
	Reference            string `json:"reference"`
	SessionID            string `json:"sessionID"`
	ChannelCode          string `json:"channelCode"`
	Status               string `json:"status"`
	AccountName          string `json:"accountName"`
	AccountNumber        string `json:"accountNumber"`
	BankVerificationCode string `json:"BankVerificationCode"`
}
