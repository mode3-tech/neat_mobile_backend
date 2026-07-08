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
	Type                             string `json:"type"`
	UserID                           string `json:"userId"`
	PaidAt                           string `json:"paidAt"`
	Amount                           string `json:"amount"`
	WalletID                         string `json:"walletId"`
	Narration                        string `json:"narration"`
	Reference                        string `json:"reference"`
	SessionID                        string `json:"sessionID"`
	ChannelCode                      string `json:"channelCode"`
	Status                           string `json:"status"`
	BeneficiaryAccountName           string `json:"beneficiaryAccountName"`
	BeneficiaryAccountNumber         string `json:"beneficiaryAccountNumber"`
	BeneficiaryBankVerificationNumber string `json:"beneficiaryBankVerificationNumber"`
	DestinationInstitutionCode       string `json:"destinationInstitutionCode"`
	OriginatorAccountName            string `json:"originatorAccountName"`
	OriginatorAccountNumber          string `json:"originatorAccountNumber"`
	OriginatorBankVerificationNumber string `json:"originatorBankVerificationNumber"`
	AccountName                      string `json:"accountName"`
	AccountNumber                    string `json:"accountNumber"`
	BankVerificationCode             string `json:"BankVerificationCode"`
}
