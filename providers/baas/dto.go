package baas

type optimusTokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type optimusTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken any    `json:"refreshToken"`
}

type optimusVerifyOTPRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	OTPToken    string `json:"otpToken"`
	Email       string `json:"email"`
	ReferenceID string `json:"referenceId"`
}

type OptimusPayload struct {
	RequestId         string `json:"RequestId"`
	Email             string `json:"Email"`
	Gender            string `json:"Gender"`
	MaritalStatus     string `json:"MaritalStatus"`
	MothersMaidenName string `json:"MothersMaidenName"`
	Address           string `json:"Address"`
	HouseNo           string `json:"HouseNo"`
	ProductId         string `json:"ProductId"`
	PhoneNumber       string `json:"PhoneNumber"`
	BVN               string `json:"Bvn"`
}
