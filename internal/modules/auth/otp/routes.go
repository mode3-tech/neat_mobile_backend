package otp

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *OTPHandler) {
	auth := rg.Group("/auth")
	{
		auth.POST("/otp/sms/request", handler.RequestSMSOTP)
		auth.POST("/otp/email/request", handler.RequestEmailOTP)
		auth.POST("/otp/verify", handler.VerifyOTP)
	}
}
