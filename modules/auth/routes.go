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
		auth.POST("/password/forgot/resend", handler.ResendForgotPasswordOTP)
		auth.PATCH("/password/reset", handler.ResetPassword)

		// Protected routes
		auth.POST("/logout", authGuard, handler.Logout)
		auth.POST("/pin/forgot", authGuard, handler.ForgotTransactionPin)
		auth.POST("/pin/forgot/resend", authGuard, handler.ResendForgotTransactionPinOTP)
		auth.PATCH("/pin/reset", authGuard, handler.ResetTransactionPin)
		auth.POST("/pin/change/request", authGuard, handler.RequestTransactionPinChange)
		auth.POST("/pin/change/resend", authGuard, handler.ResendRequestTransactionPinChangeOTP)
		auth.PATCH("/pin/change", authGuard, handler.ChangeTransactionPin)
		auth.POST("/password/change/request", authGuard, handler.RequestPasswordChange)
		auth.POST("/password/change/resend", authGuard, handler.ResendPasswordChangeOTP)
		auth.PATCH("/password/change", authGuard, handler.ChangePassword)
	}
}
