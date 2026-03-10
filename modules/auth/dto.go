package auth

type DeviceRegisteration struct {
	DeviceID    string `json:"device_id" binding:"required"`
	PublicKey   string `json:"public_key" binding:"required"`
	DeviceName  string `json:"device_name" binding:"required"`
	DeviceModel string `json:"device_model" binding:"required"`
	OS          string `json:"os" binding:"required"`
	OSVersion   string `json:"os_version" binding:"required"`
	AppVersion  string `json:"app_version" binding:"required"`
}

type RegisterRequest struct {
	PhoneNumber           string              `json:"phone_number" binding:"required"`
	Email                 string              `json:"email"`
	Password              string              `json:"password" binding:"required"`
	ConfirmPassword       string              `json:"confirm_password" binding:"required"`
	TransactionPin        string              `json:"transaction_pin" binding:"required"`
	ConfirmTransactionPin string              `json:"confirm_transaction_pin" binding:"required"`
	BVNVerificationID     string              `json:"bvn_verification_id" binding:"required"`
	NINVerificationID     string              `json:"nin_verification_id" binding:"required"`
	PhoneVerificationID   string              `json:"phone_verification_id" binding:"required"`
	EmailVerificationID   string              `json:"email_verification_id"`
	Device                DeviceRegisteration `json:"device" binding:"required"`
}

type LoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LoginInitResponse struct {
	Status       string `json:"status"`
	Challenge    string `json:"challenge,omitempty"`
	SessionToken string `json:"session_token,omitempty"`
}

type VerifyDeviceRequest struct {
	Challenge string `json:"challenge" binding:"required"`
	Signature string `json:"signature" binding:"required"`
	DeviceID  string `json:"device_id" binding:"required"`
}

type VerifyDeviceResponse struct {
	Status       string `json:"status"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ActionToken  string `json:"action_token,omitempty"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RefreshTokenRequest struct {
	DeviceID     string `json:"device_id" binding:"required"`
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type SMSOTPRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type EmailOTPRequest struct {
	Email string `json:"email" binding:"required"`
}

type BVNValidationRequest struct {
	BVN string `json:"bvn" binding:"required"`
}

type BVNValidationResponse struct {
	Name           string `json:"name"`
	DOB            string `json:"dob"`
	PhoneNumber    string `json:"phone_number"`
	VerificationID string `json:"verification_id"`
}

type NINValidationRequest struct {
	BVNValidationID string `json:"bvn_validation_id" binding:"required"`
	NIN             string `json:"nin" binding:"required"`
}

type NINValidationResponse struct {
	Name           string `json:"name"`
	DOB            string `json:"dob"`
	PhoneNumber    string `json:"phone_number"`
	VerificationID string `json:"verification_id"`
}

type NewDeviceResquest struct {
	SessionToken string              `json:"session_token" binding:"required"`
	OTP          string              `json:"otp" binding:"required"`
	Device       DeviceRegisteration `json:"device" binding:"required"`
}

type ForgotPasswordRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type ResetPasswordRequest struct {
	ResetCode string `json:"reset_code" binding:"required"`
	Password  string `json:"password" binding:"password"`
}
