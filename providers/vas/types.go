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
	RequestID       string `json:"requestId,omitempty"`
	ReferenceID     string `json:"referenceId,omitempty"`
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

type CategoriesResponse struct {
	Response
	Data categoriesResponseData `json:"data"`
}

type categoriesResponseData struct {
	HasNextRecord   bool       `json:"hasNextRecord"`
	TotalCount      int        `json:"totalCount"`
	CategoryDTOList []Category `json:"categoryDTOList"`
}

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ProductResponse struct {
	Response
	Data productResponseData `json:"data"`
}

type productResponseData struct {
	HasNextRecord   bool      `json:"hasNextRecord"`
	TotalCount      int       `json:"totalCount"`
	CategoryDTOList []Product `json:"categoryDTOList"`
}

type Product struct {
	Name        string  `json:"name"`
	UniqueCode  string  `json:"uniqueCode"`
	LookUp      bool    `json:"lookUp"`
	FixedAmount bool    `json:"fixedAmount"`
	Amount      float32 `json:"amount"`
	MinAmount   float32 `json:"minimumAmount"`
	MaxAmount   float32 `json:"maximumAmount"`
	ImageURL    string  `json:"imageUrl"`
	BillerName  string  `json:"billerName"`
	CategoryDTO struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"categoryDTO"`
}

type BillersByCategoryIDResponse struct {
	Response
	Data billersByCategoryIDResponseData `json:"data"`
}

type billersByCategoryIDResponseData struct {
	HasNextRecord bool     `json:"hasNextRecord"`
	TotalCount    int      `json:"totalCount"`
	BillerDTOList []Biller `json:"billerDTOList"`
}

type Biller struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	BillerCode   string `json:"billerCode"`
	Description  string `json:"description"`
	CategoryDTOs []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"categoryDTOS"`
	Image string `json:"image"`
}

// {
//   "requestId": "1109991201",
//   "referenceId": "MATT14539722120213323215051212",
//   "responseCode": "00",
//   "responseMessage": "Successful",
//   "data": {
//     "pin": "758324096129",
//     "serial": "W12A0001981",
//     "transactionDate": "20221202 13:32:28.856"
//   }
// }
