package otp

type RequestOTPRequest struct {
	Destination    string `json:"destination" binding:"required"`
	VerificationID string `json:"verification_id" binding:"required"`
}

type VerifyOTPRequest struct {
	VerificationID string `json:"verification_id" binding:"required"`
	OTP            string `json:"otp" binding:"required,len=6,numeric"`
}

type VerifyOTPResponse struct {
	VerificationID string `json:"verification_id" binding:"required"`
}
