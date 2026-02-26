package otp

type RequestOTPRequest struct {
	Purpose     string `json:"purpose" binding:"required,oneof=login signup password_reset"`
	Channel     string `json:"channel" binding:"required,oneof=sms email"`
	Destination string `json:"destination" binding:"required"`
}

type VerifyOTPRequest struct {
	Purpose     string `json:"purpose" binding:"required,oneof=login signup password_reset"`
	Channel     string `json:"channel" binding:"required,oneof=sms email"`
	Destination string `json:"destination" binding:"required"`
	OTP         string `json:"otp" binding:"required,len=6,numeric"`
}
