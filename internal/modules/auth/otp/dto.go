package otp

type RequestOTPRequest struct {
	Purpose        string `json:"purpose" binding:"required,oneof=login signup password_reset pin_reset"`
	Channel        string `json:"channel" binding:"required,oneof=sms email"`
	VerificationID string `json:"verification_id" binding:"required"`
}

type VerifyOTPRequest struct {
	Purpose        string `json:"purpose" binding:"required,oneof=login signup password_reset pin_reset"`
	Channel        string `json:"channel" binding:"required,oneof=sms email"`
	VerificationID string `json:"verification_id" binding:"required"`
	OTP            string `json:"otp" binding:"required,len=6,numeric"`
}

type VerifyOTPResponse struct {
	VerificationID string `json:"verification_id" binding:"required"`
}
