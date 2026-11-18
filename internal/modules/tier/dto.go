package tier

type ValidatePassportPhotographRequest struct {
	Image string `json:"image" binding:"required"`
}

type ValidatePassportPhotographResponse struct {
}

// ResidentialAddress is the address ALAT needs for a Tier 3 upgrade.
type ResidentialAddress struct {
	BuildingNumber        string `json:"building_number" binding:"required"`
	Apartment             string `json:"apartment"`
	Street                string `json:"street" binding:"required"`
	City                  string `json:"city" binding:"required"`
	Town                  string `json:"town"`
	State                 string `json:"state" binding:"required"`
	Lga                   string `json:"lga" binding:"required"`
	Lcda                  string `json:"lcda"`
	Landmark              string `json:"landmark"`
	AdditionalInformation string `json:"additional_information"`
	Country               string `json:"country" binding:"required"`
	FullAddress           string `json:"full_address" binding:"required"`
	PostalCode            string `json:"postal_code"`
}

type UpgradeTier3Request struct {
	ResidentialAddress ResidentialAddress `json:"residential_address" binding:"required"`
}
