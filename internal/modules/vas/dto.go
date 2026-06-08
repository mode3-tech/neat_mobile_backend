package vas

type AirtimePayload struct {
	RequestID   string `json:"request_id" validate:"required"`
	UniqueCode  string `json:"unique_code" validate:"required"`
	PhoneNumber string `json:"phone_number" validate:"required"`
	Amount      int64  `json:"amount" validate:"amount,gt=0"`
}

type Response struct {
	RequestID       string `json:"requestId"`
	ReferenceID     string `json:"referenceId"`
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
}

type ISPResponse struct {
	Response
	Data ispData `json:"data"`
}

type ispData struct {
	Channel     string `json:"channel"`
	Amount      int64  `json:"amount"`
	PhoneNumber string `json:"phoneNumber"`
}

type ElectricityValidationResponse struct {
	Response
	Data electricityValidationResponseData `json:"data"`
}

type electricityValidationResponseData struct {
	AccountNumber  string      `json:"accountNumber"`
	AccountType    AccountType `json:"accountType"`
	Name           string      `json:"name"`
	Address        string      `json:"address"`
	MinimumVending string      `json:"minimumVending"`
}

type PayElectricityResponse struct {
	Response
	Data payElectricityResponseData `json:"data"`
}

type payElectricityResponseData struct {
	Amount        string `json:"amount"`
	Unit          string `json:"unit"`
	Address       string `json:"address"`
	Arrears       string `json:"arrears"`
	TariffName    string `json:"tariffName"`
	Rate          string `json:"rate"`
	VAT           string `json:"vat"`
	AccountNumber string `json:"accountNumber"`
	Token         string `json:"token"`
}

type CableValidationResponse struct {
	Response
	Data cableValidationResponseData `json:"data"`
}

type cableValidationResponseData struct {
	AccountNumber string  `json:"accountNumber"`
	AccountType   string  `json:"accountType"`
	Name          string  `json:"name"`
	Amount        string  `json:"amount"`
	Balance       *string `json:"balance"`
	MD            bool    `json:"md"`
}

type PayCableResponse struct {
	Response
	Data payCableResponseData `json:"data"`
}

type payCableResponseData struct {
	AccountNumber string `json:"accountNumber"`
	Package       string `json:"package"`
	NoOfMonth     *int   `json:"noOfMonth"`
	Name          string `json:"name"`
	Amount        string `json:"amount"`
}
