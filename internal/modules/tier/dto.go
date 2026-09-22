package tier

type ValidatePassportPhotographRequest struct {
	Image string `json:"image" binding:"required"`
}

type ValidatePassportPhotographResponse struct {
	
}