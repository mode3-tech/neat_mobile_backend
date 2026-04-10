package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard gin.HandlerFunc, loginMiddlewares ...gin.HandlerFunc) {

	auth := rg.Group("/auth")

	{
		auth.POST("/register", handler.Register)

		loginHandlers := append(loginMiddlewares, handler.Login)
		auth.POST("/login", loginHandlers...)

		auth.POST("/device/challenge/verify", handler.VerifyDevice)
		auth.POST("/device/otp/verify", handler.VerifyNewDevice)
		auth.POST("/device/otp/resend", handler.ResendNewDeviceOTP)
		auth.POST("/refresh", handler.RefreshAccessToken)
		auth.POST("/validate/bvn", handler.VerifyBVN)
		auth.POST("/validate/nin", handler.VerifyNIN)
		auth.POST("/password/forgot", handler.ForgotPassword)
		auth.POST("/password/reset", handler.ResetPassword)

		// Protected route
		auth.POST("/logout", authGuard, handler.Logout)
	}
}
