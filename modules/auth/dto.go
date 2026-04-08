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
	IsBiometricsEnabled   *bool               `json:"is_biometrics_enabled" binding:"required"`
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

type ResendNewDeviceOTPRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
	DeviceID     string `json:"device_id" binding:"required"`
}

type WalletPayload struct {
	BVN         string                 `json:"bvn" binding:"required"`
	FirstName   string                 `json:"firstName" binding:"required"`
	LastName    string                 `json:"lastName" binding:"required"`
	DateOfBirth string                 `json:"dateOfBirth" binding:"required"`
	PhoneNumber string                 `json:"phoneNumber" binding:"required"`
	Email       string                 `json:"email" binding:"required,email"`
	Address     string                 `json:"address" binding:"required"`
	Metadata    map[string]interface{} `json:"metadata" binding:"required"`
}

type WalletResponse struct {
	Status   *bool           `json:"status"`
	Customer *WalletCustomer `json:"customer,omitempty"`
	Wallet   *WalletInfo     `json:"wallet,omitempty"`
}

type WalletCustomer struct {
	ID           string         `json:"id"`
	Metadata     map[string]any `json:"metadata"`
	BVN          string         `json:"bvn,omitempty"`
	Currency     string         `json:"currency,omitempty"`
	DateOfBirth  string         `json:"dateOfBirth,omitempty"`
	PhoneNumber  string         `json:"phoneNumber,omitempty"`
	LastName     string         `json:"lastName,omitempty"`
	FirstName    string         `json:"firstName,omitempty"`
	BVNLastName  string         `json:"BVNLastName,omitempty"`
	BVNFirstName string         `json:"BVNFirstName,omitempty"`
	NameMatch    *bool          `json:"nameMatch,omitempty"`
	Email        string         `json:"email,omitempty"`
	Mode         string         `json:"mode,omitempty"`
	MerchantId   string         `json:"MerchantId,omitempty"`
	Tier         string         `json:"tier,omitempty"`
	UpdatedAt    string         `json:"updatedAt,omitempty"`
	CreatedAt    string         `json:"createdAt,omitempty"`
	Address      *string        `json:"address,omitempty"`
}

type WalletInfo struct {
	ID               string `json:"id"`
	Mode             string `json:"mode"`
	Email            string `json:"email"`
	Currency         string `json:"currency"`
	BankName         string `json:"bankName"`
	BankCode         string `json:"bankCode"`
	AccountName      string `json:"accountName"`
	AccountNumber    string `json:"accountNumber"`
	AccountReference string `json:"accountReference"`
	UpdatedAt        string `json:"updatedAt"`
	CreatedAt        string `json:"createdAt"`
	BookedBalance    int64  `json:"bookedBalance"`
	AvailableBalance int64  `json:"availableBalance"`
	Status           string `json:"status"`
	Updated          bool   `json:"updated"`
	WalletType       string `json:"walletType"`
	WalletId         string `json:"walletId"`
}

type CBAWalletUpdate struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	Bank          string `json:"bank"`
	BankCode      string `json:"bank_code"`
}
