package otp

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *OTPHandler) {
	auth := rg.Group("/auth")
	{
		auth.POST("/otp/request", handler.RequestOTP)
		auth.POST("/otp/verify", handler.VerifyOTP)
	}
}
