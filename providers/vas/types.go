package vas

type AccountType string

const (
	AccountTypePrepaid  AccountType = "prepaid"
	AccountTypePostpaid AccountType = "postpaid"
)

type Payload struct {
	RequestID  string `json:"requestId"`
	UniqueCode string `json:"uniqueCode"`
}

type Response struct {
	RequestID       string `json:"requestId"`
	ReferenceID     string `json:"referenceId"`
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
}

type ISPPayload struct {
	Payload
	Details ispDetails `json:"details"`
}

type ispDetails struct {
	PhoneNumber string `json:"phoneNumber"`
	Amount      int64  `json:"amount"`
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

type ElectricityValidationPayload struct {
	Payload
	Details electricityValidationDetails `json:"details"`
}

type electricityValidationDetails struct {
	AccountNumber string      `json:"accountNumber"`
	AccountType   AccountType `json:"accountType"`
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

type PayElectricityBillPayload struct {
	Payload
	Details payElectricityBillDetails `json:"details"`
}

type payElectricityBillDetails struct {
	AccountNumber string      `json:"accountNumber"`
	AccountType   AccountType `json:"accountType"`
	Amount        int64       `json:"amount"`
	Name          string      `json:"name"`
	Address       string      `json:"address"`
	PhoneNumber   string      `json:"phoneNumber"`
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

type CableValidationPayload struct {
	Payload
	Details cableValidationDetails `json:"details"`
}

type cableValidationDetails struct {
	AccountNumber string `json:"accountNumber"`
	NoOfMonth     int    `json:"noOfMonth"`
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

type PayCableBillPayload struct {
	Payload
	Details payCableBillDetails `json:"details"`
}

type payCableBillDetails struct {
	AccountNumber string `json:"accountNumber"`
	AccountType   string `json:"accountType"`
	NoOfMonth     int    `json:"noOfMonth"`
	Amount        int64  `json:"amount"`
	Name          string `json:"name"`
	PhoneNumber   string `json:"phoneNumber"`
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
