package card

type OptimusCardRequest struct {
	ReferenceID          string `json:"referenceId"`
	CardType             string `json:"cardType"`
	AccountToLink        string `json:"accountToLink"`
	AccountToDebit       string `json:"accountToDebit"`
	Reason               string `json:"reason"`
	IsBranchPickup       bool   `json:"isBranchPickup"`
	BranchPickupLocation string `json:"branchPickupLocation"`
	IsHomeDelivery       bool   `json:"isHomeDelivery"`
	HouseNumber          string `json:"houseNumber"`
	StreetName           string `json:"streetName"`
	City                 string `json:"city"`
	State                string `json:"state"`
	LGA                  string `json:"lga"`
	DeliveryFee          int64  `json:"deliveryFee"`
}

type OptimusCardResponse struct {
	ResponseMessage string `json:"responseMessage"`
	ResponseCode    string `json:"responseCode"`
	Data            any    `json:"data"`
}

// {
//   "responseMessage": "fugiat",
//   "responseCode": "ut proident ad fug",
//   "data": {
//     "nullable": true
//   }
// }
