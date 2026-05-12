package card

type RequestForCardRequest struct {
	IsBranchPickup       *bool  `json:"is_branch_pickup"`
	BranchPickupLocation string `json:"branch_pickup_location"`
	IsHomeDelivery       *bool  `json:"is_home_delivery"`
	HouseNumber          string `json:"house_number"`
	StreetName           string `json:"street_name"`
	City                 string `json:"city"`
	State                string `json:"state"`
	LGA                  string `json:"lga"`
	DeliveryFee          int64  `json:"delivery_fee" binding:"required"`
}
