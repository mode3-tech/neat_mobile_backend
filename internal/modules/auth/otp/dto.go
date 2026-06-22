package otp

type RequestSMSOTPRequest struct {
	VerificationID string `json:"verification_id" binding:"required"`
}

type RequestEmailOTPRequest struct {
	VerificationID string `json:"verification_id" binding:"required"`
	Destination    string `json:"destination" binding:"required,email"`
}

type VerifyOTPRequest struct {
	VerificationID string `json:"verification_id" binding:"required"`
	OTP            string `json:"otp" binding:"required,len=6,numeric"`
}

type VerifyOTPResponse struct {
	VerificationID string `json:"verification_id" binding:"required"`
}
