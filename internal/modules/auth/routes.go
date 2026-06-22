package auth

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, handler *Handler, authGuard, deviceValidator gin.HandlerFunc, loginMiddlewares ...gin.HandlerFunc) {

	auth := rg.Group("/auth")

	{
		auth.POST("/register", handler.Register)
		auth.GET("/register/:job_id/status", handler.GetRegistrationStatus)
		auth.POST("/register/:job_id/claim", handler.ClaimRegistrationSession)

		loginHandlers := append(loginMiddlewares, handler.Login)
		auth.POST("/login", loginHandlers...)

		auth.POST("/device/challenge/verify", handler.VerifyDevice)
		auth.POST("/device/otp/verify", handler.VerifyNewDevice)
		auth.POST("/device/otp/resend", handler.ResendNewDeviceOTP)
		auth.POST("/refresh", handler.RefreshAccessToken)
		auth.POST("/validate/bvn", handler.VerifyBVN)
		auth.POST("/validate/bvn-with-face", handler.VerifyBVNWithFace)
		auth.POST("/validate/nin", handler.VerifyNIN)
		auth.POST("/validate/nin-with-face", handler.VerifyNINWithFace)
		auth.POST("/password/forgot", handler.ForgotPassword)
		auth.POST("/password/forgot/resend", handler.ResendForgotPasswordOTP)
		auth.POST("/password/forgot/verify", handler.VerifyForgotPasswordOTP)
		auth.PATCH("/password/reset", handler.ResetPassword)
	}

	{
		// Protected routes
		auth.POST("/logout", authGuard, handler.Logout)
		auth.POST("/pin/forgot", authGuard, deviceValidator, handler.ForgotTransactionPin)
		auth.POST("/pin/forgot/resend", authGuard, deviceValidator, handler.ResendForgotTransactionPinOTP)
		auth.POST("/pin/forgot/verify", authGuard, deviceValidator, handler.VerifyForgotTransactionPinOTP)
		auth.PATCH("/pin/reset", authGuard, deviceValidator, handler.ResetTransactionPin)
		auth.POST("/pin/change/request", authGuard, deviceValidator, handler.RequestTransactionPinChange)
		auth.POST("/pin/change/resend", authGuard, deviceValidator, handler.ResendRequestTransactionPinChangeOTP)
		auth.POST("/pin/change/verify", authGuard, deviceValidator, handler.VerifyTransactionPinChangeOTP)
		auth.PATCH("/pin/change", authGuard, deviceValidator, handler.ChangeTransactionPin)
		auth.POST("/password/change/request", authGuard, deviceValidator, handler.RequestPasswordChange)
		auth.POST("/password/change/resend", authGuard, deviceValidator, handler.ResendPasswordChangeOTP)
		auth.POST("/password/change/verify", authGuard, deviceValidator, handler.VerifyPasswordChangeOTP)
		auth.PATCH("/password/change", authGuard, deviceValidator, handler.ChangePassword)
		auth.PATCH("/biometrics/toggle", authGuard, deviceValidator, handler.ToggleBiometrics)
		auth.POST("/challenge/request", handler.ChallengeRequest)
	}
}
