package otp

type RequestSMSOTPRequest struct {
	VerificationID string `json:"verification_id" binding:"omitempty"`
	Destination    string `json:"destination" binding:"omitempty"`
	Purpose        string `json:"purpose" binding:"omitempty,oneof=signup submitted_contact"`
}

type RequestEmailOTPRequest struct {
	VerificationID string `json:"verification_id" binding:"omitempty"`
	Destination    string `json:"destination" binding:"omitempty,email"`
	Purpose        string `json:"purpose" binding:"omitempty,oneof=signup submitted_contact"`
}

type RequestOTPResponse struct {
	OTPID string `json:"otp_id"`
}

type VerifyOTPRequest struct {
	OTPID   string `json:"otp_id" binding:"required"`
	OTP     string `json:"otp" binding:"required,len=6,numeric"`
	Purpose string `json:"purpose" binding:"omitempty,oneof=signup submitted_contact"`
}

type VerifyOTPResponse struct {
	VerificationID string `json:"verification_id" binding:"required"`
}
