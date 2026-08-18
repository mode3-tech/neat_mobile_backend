package registerv2

import "github.com/gin-gonic/gin"

// RegisterRoutes exposes the Optimus onboarding verification flow. These
// endpoints are deliberately public because they run before an account exists.
func RegisterRoutes(rg *gin.RouterGroup, handler *Handler) {
	optimus := rg.Group("/auth")
	{
		optimus.POST("/otp/phone/request", handler.RequestPhoneOTP)
		optimus.POST("/otp/phone/verify", handler.VerifyPhoneOTP)
		optimus.POST("/otp/email/request", handler.RequestEmailOTP)
		optimus.POST("/otp/email/verify", handler.VerifyEmailOTP)
		optimus.POST("/validate/bvn", handler.ValidateBVN)
		optimus.POST("/validate/nin", handler.ValidateNIN)
		optimus.POST("/otp/verify", handler.VerifyOTP)
		optimus.POST("/otp/resend", handler.ResendOTP)
		optimus.POST("/register", handler.Register)
	}
}
