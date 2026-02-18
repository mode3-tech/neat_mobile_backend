package auth

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
