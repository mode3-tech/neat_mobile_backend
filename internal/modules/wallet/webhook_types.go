package wallet

import "time"

type BaaSEventEnvelope struct {
	Event string `json:"event"`
}

type CustomerBankTransferWebhook struct {
	Event string                   `json:"event"`
	Data  CustomerBankTransferData `json:"data"`
}

type CustomerBankTransferData struct {
	Amount               float64                `json:"amount"`
	PaidAt               time.Time              `json:"paidAt"`
	Charges              float64                `json:"charges"`
	SessionID            string                 `json:"sessionId"`
	VAT                  float64                `json:"vat"`
	WalletID             string                 `json:"walletId"`
	Reference            string                 `json:"reference"`
	CustomerID           string                 `json:"customerId"`
	Status               string                 `json:"status"`
	Total                float64                `json:"total"`
	TransactionReference string                 `json:"transactionReference"`
	MerchantID           string                 `json:"merchantId"`
	TransactionID        string                 `json:"transactionId"`
	StampDutyApplied     bool                   `json:"stampDutyApplied"`
	StampDutyAmount      float64                `json:"stampDutyAmount"`
	Metadata             map[string]interface{} `json:"metadata"`
	Description          string                 `json:"description"`
	Destination          string                 `json:"destination"`
}

// 2026/07/10 09:25:34 baas webhook: customer bank transfer: {"event":"customer_bank_transfer","data":{"amount":100,"paidAt":"2026-07-10T09:25:32.563Z","charges":10,"sessionID":"100040260710092532666853729598","vat":0.75,"walletId":"dd16d411-1cd7-4a6b-951e-ada02ee8bc81","reference":"HPpaMkO7DPtww0Yac90LGR5lVl2TKj","customerId":"b3aef2d4-16ff-4cb8-8040-8a19c3784167","status":"success","total":110.75,"transactionReference":"HPpaMkO7DPtww0Yac90LGR5lVl2TKj","merchantId":"844667ea-1b9a-402b-9148-d999f330ea41","transactionId":"e4cb1791-580a-445d-a28f-e0e25c82583c","description":"Transfer of NGN100.00 to ABDULSALAM IBRAHIM KOLADE (2370962139/UNITED BANK FOR AFRICA)/100040260710092532666853729598","destination":"2370962139/000004","stampDutyApplied":false,"stampDutyAmount":0,"metadata":{"amount":100,"charges":10,"bankName":"UNITED BANK FOR AFRICA","sortCode":"000004","vat":0.75,"walletId":"dd16d411-1cd7-4a6b-951e-ada02ee8bc81","narration":"testing","customerId":"b3aef2d4-16ff-4cb8-8040-8a19c3784167","accountName":"ABDULSALAM IBRAHIM KOLADE","totalAmount":110.75,"accountNumber":"2370962139","transferRoute":"NIP","walletAccountName":"Abdulbasit Adebajo","additionalMetadata":{},"walletAccountNumber":"8891450872","merchantId":"844667ea-1b9a-402b-9148-d999f330ea41","stampDutyApplied":false,"stampDutyAmount":0,"nameEnquiryRef":"100040260701091734871967757295"}}}

type AccountFundedWebhook struct {
	Event string            `json:"event"`
	Data  AccountFundedData `json:"data"`
}

type AccountFundedData struct {
	Type                              string    `json:"type"`
	UserID                            string    `json:"userId"`
	PaidAt                            time.Time `json:"paidAt"`
	Amount                            string    `json:"amount"`
	WalletID                          string    `json:"walletId"`
	Narration                         string    `json:"narration"`
	Reference                         string    `json:"reference"`
	SessionID                         string    `json:"sessionID"`
	ChannelCode                       string    `json:"channelCode"`
	Status                            string    `json:"status"`
	BeneficiaryAccountName            string    `json:"beneficiaryAccountName"`
	BeneficiaryAccountNumber          string    `json:"beneficiaryAccountNumber"`
	BeneficiaryBankVerificationNumber string    `json:"beneficiaryBankVerificationNumber"`
	DestinationInstitutionCode        string    `json:"destinationInstitutionCode"`
	OriginatorAccountName             string    `json:"originatorAccountName"`
	OriginatorAccountNumber           string    `json:"originatorAccountNumber"`
	OriginatorBankVerificationNumber  string    `json:"originatorBankVerificationNumber"`
	AccountName                       string    `json:"accountName"`
	AccountNumber                     string    `json:"accountNumber"`
	BankVerificationCode              string    `json:"BankVerificationCode"`
}
