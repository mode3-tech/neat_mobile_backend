package registerv2

type DeviceRegisteration struct {
	DeviceID    string `json:"device_id" binding:"required"`
	PublicKey   string `json:"public_key" binding:"required"`
	DeviceName  string `json:"device_name" binding:"required"`
	DeviceModel string `json:"device_model" binding:"required"`
	OS          string `json:"os" binding:"required"`
	OSVersion   string `json:"os_version" binding:"required"`
	AppVersion  string `json:"app_version" binding:"required"`
}

type OptimusRegisterRequest struct {
	FirstName             string              `json:"first_name" binding:"required"`
	LastName              string              `json:"last_name" binding:"required"`
	Dob                   string              `json:"dob" binding:"required"`
	Email                 string              `json:"email" binding:"required"`
	EmailVerificationID   string              `json:"email_verification_id" binding:"required"`
	Gender                string              `json:"gender" binding:"required"`
	MaritalStatus         string              `json:"marital_status" binding:"required"`
	MothersMaidenName     string              `json:"mothers_maiden_name" binding:"required"`
	Address               string              `json:"address" binding:"required"`
	HouseNo               string              `json:"house_no" binding:"required"`
	PhoneNumber           string              `json:"phone_number" binding:"required"`
	PhoneVerificationID   string              `json:"phone_verification_id" binding:"required"`
	BVN                   string              `json:"bvn" binding:"required"`
	NIN                   string              `json:"nin" binding:"required,len=11,numeric"`
	Password              string              `json:"password" binding:"required"`
	ConfirmPassword       string              `json:"confirm_password" binding:"required"`
	TransactionPin        string              `json:"transaction_pin" binding:"required"`
	ConfirmTransactionPin string              `json:"confirm_transaction_pin" binding:"required"`
	IsBiomtricsEnabled    bool                `json:"is_biomtrics_enabled" binding:"required"`
	ReferralCode          string              `json:"referral_code" binding:"omitempty"`
	Device                DeviceRegisteration `json:"device" binding:"required"`
}

type RegisterResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// OptimusPhoneValidationRequest starts verification of the phone number the
// customer wants us to use for communication. It may differ from the BVN phone.
type OptimusPhoneValidationRequest struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
}

// RequestPhoneOTPRequest requests an OTP be sent to the given phone number.
type RequestPhoneOTPRequest struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
}

// RequestPhoneOTPResponse carries the OTP ID needed to verify the code.
type RequestPhoneOTPResponse struct {
	OTPID string `json:"otp_id"`
}

// VerifyPhoneOTPRequest confirms the code sent by RequestPhoneOTPRequest.
type VerifyPhoneOTPRequest struct {
	OTPID string `json:"otp_id" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// OptimusEmailValidationRequest starts verification of the customer's email.
type OptimusEmailValidationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type OptimusBVNValidationRequest struct {
	RequestId    string `json:"RequestId" binding:"-"`
	Bvn          string `json:"Bvn" binding:"required,len=11,numeric"`
	Dob          string `json:"Dob" binding:"required"`
	Email        string `json:"Email" binding:"required,email"`
	ReferralCode string `json:"ReferralCode" binding:"omitempty"`
}

type OptimusNINValidationRequest struct {
	RequestId    string `json:"RequestId" binding:"-"`
	Nin          string `json:"Nin" binding:"required,len=11,numeric"`
	FirstName    string `json:"FirstName" binding:"required"`
	LastName     string `json:"LastName" binding:"required"`
	Gender       string `json:"Gender" binding:"required"`
	Dob          string `json:"Dob" binding:"required"`
	Email        string `json:"Email" binding:"required,email"`
	ReferralCode string `json:"ReferralCode" binding:"omitempty"`
}

type OptimusVerificationResponse struct {
	VerificationID string `json:"verification_id"`
	// ProviderReferenceID is Optimus's own reference id for this check (from the
	// response's Data field). Only present for BVN/NIN validation, since that's
	// what starts an Optimus OTP challenge - empty/omitted for other verification
	// types (email, phone). The client needs it for /otp/verify and /otp/resend.
	ProviderReferenceID string `json:"provider_reference_id,omitempty"`
}

// VerifyOTPRequest confirms the OTP Optimus sent for a given reference id
// (e.g. after BVN/NIN validation). Generic: works for any Optimus OTP
// challenge, not tied to a specific verification type.
type VerifyOTPRequest struct {
	PhoneNo     string `json:"phone_no" binding:"required"`
	OTPToken    string `json:"otp_token" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	ReferenceID string `json:"reference_id" binding:"required"`
}

// ResendOTPRequest asks Optimus to resend the OTP tied to a reference id.
type ResendOTPRequest struct {
	ReferenceID string `json:"reference_id" binding:"required"`
}

type OptimusResponse struct {
	Data            string `json:"Data"`
	ResponseMessage string `json:"ResponseMessage"`
	ResponseCode    string `json:"ResponseCode"`
	Error           any    `json:"Error"`
}
