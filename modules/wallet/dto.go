package wallet

import "time"

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

type InitiatedDepositRequest struct {
	ExpectedAmount int64 `json:"expected_amount" binding:"omitempty,gt=0"`
}

type InitiatedDepositResponse struct {
	Status     bool       `json:"status" binding:"required"`
	TrackingID string     `json:"tracking_id" binding:"required"`
	ExpiresAt  time.Time  `json:"expires_at" binding:"required"`
	Account    AccountObj `json:"account" binding:"required"`
}

type AccountObj struct {
	AccountNumber string `json:"account_number" binding:"required"`
	AccountName   string `json:"account_name" binding:"required"`
	BankName      string `json:"bank_name" binding:"required"`
	BankCode      string `json:"bank_code" binding:"required"`
}

type PolledDepositResponse struct {
	Status bool `json:"status" binding:"status"`
}

type DepositObj struct {
	TrackingID     string                `json:"tracking_id" binding:"required"`
	Status         ExpectedDepositStatus `json:"status" binding:"required"`
	ExpectedAmount int64                 `json:"expected_amount" binding:"required"`
	ActualAmount   int64                 `json:"actual_amount" binding:"required"`
	ExpiresAt      time.Time             `json:"expires_at" binding:"required"`
	Transaction    TransactionObj        `json:"transaction" binding:"required"`
}

type TransactionObj struct {
	ID        string    `json:"id" binding:"required"`
	Amount    int64     `json:"amount" binding:"required"`
	Reference string    `json:"reference" binding:"required"`
	Narration string    `json:"narration" binding:"required"`
	CreatedAt time.Time `json:"created_at" binding:"required"`
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

//             {
//                 "amount": 500,
//                 "vat": 0.7875,
//                 "sortCode": "000013",
//                 "reference": "l9gdbqe7FmX06KjuZt0r0seAJ69nHykXTHvM",
//                 "narration": "Just kidding",
//                 "accountName": "Obagunwa Emmanuel",
//                 "fee": 10.5,
//                 "accountNumber": "0167421242",
//                 "total": 511.2875
//             },
//             {
//                 "amount": 250,
//                 "vat": 0.7875,
//                 "sortCode": "000013",
//                 "reference": "MBg5hvkWlSMXyCTsjtYw3MIBSmWMMp77CfAu",
//                 "narration": "Just kidding",
//                 "accountName": "Obagunwa Emmanuel",
//                 "fee": 10.5,
//                 "accountNumber": "0167421242",
//                 "total": 261.2875
//             }
//         ],
//         "rejected": [],
//         "accepted": [
//             {
//                 "amount": 500,
//                 "vat": 0.7875,
//                 "bankName": "GTBANK PLC",
//                 "sortCode": "000013",
//                 "metadata": {
//                     "sessionId": "187923755680432213004147647494",
//                     "transactionReference": "8828075379951199"
//                 },
//                 "reference": "l9gdbqe7FmX06KjuZt0r0seAJ69nHykXTHvM",
//                 "narration": "Just kidding",
//                 "accountName": "Obagunwa Emmanuel",
//                 "fee": 10.5,
//                 "accountNumber": "0167421242",
//                 "total": 511.2875
//             },
//             {
//                 "amount": 250,
//                 "vat": 0.7875,
//                 "bankName": "GTBANK PLC",
//                 "sortCode": "000013",
//                 "metadata": {
//                     "sessionId": "026545050219349897184258832943",
//                     "transactionReference": "8341434570354439"
//                 },
//                 "reference": "MBg5hvkWlSMXyCTsjtYw3MIBSmWMMp77CfAu",
//                 "narration": "Just kidding",
//                 "accountName": "Obagunwa Emmanuel",
//                 "fee": 10.5,
//                 "accountNumber": "0167421242",
//                 "total": 261.2875
//             }
//         ]
//     }
// }
