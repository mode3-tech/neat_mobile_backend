package auth

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
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
	Name        string `json:"name"`
	DOB         string `json:"dob"`
	PhoneNumber string `json:"phone_number"`
}
