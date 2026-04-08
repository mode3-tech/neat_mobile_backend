package internal

type CBAStatus string

const (
	CBAStatusFailed     CBAStatus = "failed"
	CBAStatusSuccessful CBAStatus = "successful"
)

type CustomerUpdateRequest struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	Bank          string `json:"bank"`
	BankCode      string `json:"bank_code"`
}

type CustomerUpdateResponse struct {
	Status  CBAStatus `json:"status"`
	Message string    `json:"message"`
}
