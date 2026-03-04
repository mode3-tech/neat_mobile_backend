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
	PhoneNumber         string              `json:"phone_number" binding:"required"`
	Email               string              `json:"email" binding:"email"`
	Password            string              `json:"password" binding:"required,password"`
	ConfirmPassword     string              `json:"confirm_password" binding:"required,password"`
	TransactionPin      string              `json:"transaction_pin" binding:"required"`
	BVNVerificationID   string              `json:"bvn_verification_id" binding:"required"`
	NINVerificationID   string              `json:"nin_verification_id" binding:"required"`
	PhoneVerificationID string              `json:"phone_verification_id" binding:"required"`
	EmailVerificationID string              `json:"email_verification_id"`
	Device              DeviceRegisteration `json:"device" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,password"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RefreshTokenRequest struct {
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
	NIN string `json:"nin" binding:"required"`
}

type NINValidationResponse struct {
	Name           string `json:"name"`
	DOB            string `json:"dob"`
	PhoneNumber    string `json:"phone_number"`
	VerificationID string `json:"verification_id"`
}
